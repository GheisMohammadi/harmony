package stagedsync

import "errors"

// Errors ...
var (
	ErrRegistrationFail                   = errors.New("registration failed")
	ErrGetBlock                           = errors.New("get block failed")
	ErrFetchBlockHashProgressFail         = errors.New("fetch cache progress for block hashes stage failed")
	ErrFetchCachedBlockHashFail           = errors.New("fetch cached block hashes failed")
	ErrNotEnoughBlockHashes               = errors.New("peers haven't sent all requested block hashes")
	ErrRetrieveCachedProgressFail         = errors.New("retrieving cache progress for block hashes stage failed")
	ErrRetrieveCachedHashProgressFail     = errors.New("retrieving cache progress for block hashes stage failed")
	ErrSaveBlockHashesProgressFail        = errors.New("saving progress for block hashes stage failed")
	ErrSaveCachedBlockHashesProgressFail  = errors.New("saving cache progress for block hashes stage failed")
	ErrSavingCacheLastBlockHashFail       = errors.New("saving cache last block hash for block hashes stage failed")
	ErrCachingBlockHashFail               = errors.New("caching downloaded block hashes failed")
	ErrCommitTransactionFail              = errors.New("failed to write db commit")
	ErrUnexpectedNumberOfBlocks           = errors.New("unexpected number of block delivered")
	ErrSavingBodiesProgressFail           = errors.New("saving progress for block bodies stage failed")
	ErrAddTasksToQueueFail                = errors.New("cannot add task to queue")
	ErrSavingCachedBodiesProgressFail     = errors.New("saving cache progress for blocks stage failed")
	ErrRetrievingCachedBodiesProgressFail = errors.New("retrieving cache progress for blocks stage failed")
	ErrNoConnectedPeers                   = errors.New("haven't connected to any peer yet")
	ErrSaveStateProgressFail              = errors.New("saving progress for block States stage failed")
)
