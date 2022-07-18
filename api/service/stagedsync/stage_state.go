package stagedsync

import (
	"context"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/pkg/errors"
)

type StageStates struct {
	configs StageStatesCfg
}
type StageStatesCfg struct {
	ctx context.Context
	bc  *core.BlockChain
	db  kv.RwDB
}

func NewStageStates(cfg StageStatesCfg) *StageStates {
	return &StageStates{
		configs: cfg,
	}
}

func NewStageStatesCfg(ctx context.Context, bc *core.BlockChain, db kv.RwDB) StageStatesCfg {
	return StageStatesCfg{
		ctx: ctx,
		bc:  bc,
		db:  db,
	}
}

// ExecStatesStage progresses States stage in the forward direction
func (stg *StageStates) Exec(firstCycle bool, badBlockUnwind bool, s *StageState, unwinder Unwinder, tx kv.RwTx) (err error) {

	currProgress := uint64(0)
	targetHeight := uint64(0)
	isBeacon := s.state.isBeacon

	if errV := stg.configs.db.View(stg.configs.ctx, func(etx kv.Tx) error {
		if targetHeight, err = GetStageProgress(etx, BlockBodies, isBeacon); err != nil {
			return err
		}
		// if currProgress, err = GetStageProgress(etx, States, isBeacon); err != nil {
		// 	return err
		// }
		// if currProgress == 0 {
		currProgress = stg.configs.bc.CurrentBlock().NumberU64()
		//}
		return nil
	}); errV != nil {
		return errV
	}

	if currProgress >= targetHeight {
		s.state.Done()
		return nil
	}

	//size := uint64(0)

	r, err := stg.configs.db.BeginRo(stg.configs.ctx)
	if err != nil {
		return err
	}
	defer r.Rollback()
	bucketName := GetBucketName(DownloadedBlocksBucket, s.state.isBeacon)
	c, err := r.CursorDupSort(bucketName)
	if err != nil {
		return err
	}

	firstKey := fmt.Sprintf("%020d", currProgress+1)

	verifyAllSig := true //TODO: move to configs

	// update current progress before return
	// defer stg.saveProgress(s, nil)

	fmt.Print("\033[s") // save the cursor position
	for n, blockBytes, err := c.Seek([]byte(firstKey)); n != nil; n, blockBytes, err = c.Next() {
		fmt.Print("\033[u\033[K") // restore the cursor position and clear the line
		if err != nil {
			return err
		}
		sz := len(blockBytes)
		if sz <= 1 {
			continue
		}

		block, err := RlpDecodeBlockOrBlockWithSig(blockBytes)
		if err != nil {
			utils.Logger().Warn().
				Err(err).
				Str("block number", string(n)).
				Msg("block RLP decode failed")
			// TODO: handle bad block
			return err
		}
		// gotHash := block.Hash()
		// if !bytes.Equal(gotHash[:], tasks[i].blockHash) {
		// 	utils.Logger().Warn().
		// 		Err(errors.New("wrong block delivery")).
		// 		Str("expectHash", hex.EncodeToString(tasks[i].blockHash)).
		// 		Str("gotHash", hex.EncodeToString(gotHash[:]))
		// 	continue
		// }

		if block.NumberU64() <= currProgress {
			utils.Logger().Warn().
				Err(err).
				Uint64("block number", block.NumberU64()).
				Uint64("current head", currProgress).
				Msg("block number doesn't match with current head")
			fmt.Print("\033[s")
			continue
		}

		// Verify block signatures
		if block.NumberU64() > 1 {
			// Verify signature every 100 blocks
			haveCurrentSig := len(block.GetCurrentCommitSig()) != 0
			verifySeal := block.NumberU64()%verifyHeaderBatchSize == 0 || verifyAllSig
			verifyCurrentSig := verifyAllSig && haveCurrentSig
			bc := stg.configs.bc
			if err = stg.verifyBlockSignatures(bc, block, verifyCurrentSig, verifySeal, verifyAllSig); err != nil {
				// TODO: handle bad block
				return err
			}
		}

		// Verify block signatures

		// if block.NumberU64() > 1 {

		// 	verifySeal := block.NumberU64()%verifyHeaderBatchSize == 0 || verifyAllSig

		// 	err := stg.configs.bc.Engine().VerifyHeader(stg.configs.bc, block.Header(), verifySeal)
		// 	if err == engine.ErrUnknownAncestor {
		// 		return err
		// 	} else if err != nil {
		// 		utils.Logger().Error().Err(err).Msgf("[STAGED_SYNC] failed verifying signatures for new block %d", block.NumberU64())

		// 		if !verifyAllSig {
		// 			utils.Logger().Info().Interface("block", stg.configs.bc.CurrentBlock()).Msg("[STAGED_SYNC] Rolling back last 99 blocks!")
		// 			for i := uint64(0); i < verifyHeaderBatchSize-1; i++ {
		// 				if rbErr := stg.configs.bc.Rollback([]common.Hash{stg.configs.bc.CurrentBlock().Hash()}); rbErr != nil {
		// 					utils.Logger().Err(rbErr).Msg("[STAGED_SYNC] UpdateBlockAndStatus: failed to rollback")
		// 					return err
		// 				}
		// 			}

		// 			currProgress = stg.configs.bc.CurrentBlock().NumberU64()
		// 		}
		// 		return err
		// 	}
		// }
		_, err = stg.configs.bc.InsertChain([]*types.Block{block}, false /* verifyHeaders */)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msgf(
					"[STAGED_SYNC] UpdateBlockAndStatus: Error adding new block to blockchain %d %d",
					block.NumberU64(),
					block.ShardID(),
				)
			return err
		}
		utils.Logger().Info().
			Uint64("blockHeight", block.NumberU64()).
			Uint64("blockEpoch", block.Epoch().Uint64()).
			Str("blockHex", block.Hash().Hex()).
			Uint32("ShardID", block.ShardID()).
			Msg("[STAGED_SYNC] UpdateBlockAndStatus: New Block Added to Blockchain")

		// update cur progress
		currProgress = stg.configs.bc.CurrentBlock().NumberU64()

		for i, tx := range block.StakingTransactions() {
			utils.Logger().Info().
				Msgf(
					"StakingTxn %d: %s, %v", i, tx.StakingType().String(), tx.StakingMessage(),
				)
		}

		fmt.Println("insert blocks progress:", currProgress, "/", targetHeight)
	}

	return nil
}

//verifyBlockSignatures verifies block signatures
func (stg *StageStates) verifyBlockSignatures(bc *core.BlockChain, block *types.Block, verifyCurrentSig bool, verifySeal bool, verifyAllSig bool) (err error) {
	if verifyCurrentSig {
		sig, bitmap, err := chain.ParseCommitSigAndBitmap(block.GetCurrentCommitSig())
		if err != nil {
			return errors.Wrap(err, "parse commitSigAndBitmap")
		}

		startTime := time.Now()
		if err := bc.Engine().VerifyHeaderSignature(bc, block.Header(), sig, bitmap); err != nil {
			return errors.Wrapf(err, "verify header signature %v", block.Hash().String())
		}
		utils.Logger().Debug().Int64("elapsed time", time.Now().Sub(startTime).Milliseconds()).Msg("[STAGED_SYNC] VerifyHeaderSignature")
	}
	// err = bc.Engine().VerifyHeader(bc, block.Header(), verifySeal)
	// if err == engine.ErrUnknownAncestor {
	// 	return err
	// } else if err != nil {
	// 	utils.Logger().Error().Err(err).Msgf("[STAGED_SYNC] UpdateBlockAndStatus: failed verifying signatures for new block %d", block.NumberU64())

	// 	if !verifyAllSig {
	// 		utils.Logger().Info().Interface("block", bc.CurrentBlock()).Msg("[STAGED_SYNC] UpdateBlockAndStatus: Rolling back last 99 blocks!")
	// 		for i := uint64(0); i < verifyHeaderBatchSize-1; i++ {
	// 			if rbErr := bc.Rollback([]common.Hash{bc.CurrentBlock().Hash()}); rbErr != nil {
	// 				utils.Logger().Err(rbErr).Msg("[STAGED_SYNC] UpdateBlockAndStatus: failed to rollback")
	// 				return err
	// 			}
	// 		}
	// 	}
	// 	return err
	// }
	return nil
}

func (stg *StageStates) saveProgress(s *StageState, tx kv.RwTx) (err error) {

	useExternalTx := tx != nil
	if !useExternalTx {
		var err error
		tx, err = stg.configs.db.BeginRw(context.Background())
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// save progress
	if err = s.Update(tx, stg.configs.bc.CurrentBlock().NumberU64()); err != nil {
		utils.Logger().Info().
			Msgf("[STAGED_SYNC] saving progress for block States stage failed: %v", err)
		return fmt.Errorf("saving progress for block States stage failed")
	}

	if !useExternalTx {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (stg *StageStates) Unwind(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = stg.configs.db.BeginRw(stg.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	// MakeStatesNonCanonical

	if err = u.Done(tx); err != nil {
		return err
	}
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (stg *StageStates) Prune(firstCycle bool, p *PruneState, tx kv.RwTx) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = stg.configs.db.BeginRw(stg.configs.ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
