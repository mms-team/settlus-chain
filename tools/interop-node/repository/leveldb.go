package repository

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"

	sdkmath "cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/common"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/settlus/chain/tools/interop-node/types"
)

const (
	BLOCK_HASH_PREFIX    = "BH"
	BLOCK_NUMBER_PREFIX  = "BN"
	NFT_OWNERSHIP_PREFIX = "NO"
)

type LevelDbRepository struct {
	db *leveldb.DB
}

var _ Repository = (*LevelDbRepository)(nil)

func NewLevelDbRepostiory(path string, chainId string) *LevelDbRepository {
	db, err := leveldb.OpenFile(filepath.Join(path, chainId), nil)
	if err != nil {
		panic(err)
	}

	return &LevelDbRepository{db: db}
}

// GetBlockNumber implements Repository.
func (repo *LevelDbRepository) GetBlockNumber(blockHash string) (string, error) {
	blockHashBytes := common.FromHex(blockHash)
	value, err := repo.db.Get(hashKey(blockHashBytes), nil)
	if err != nil {
		return "", err
	}
	return common.Bytes2Hex(value), nil
}

// GetBlockHash implements Repository.
func (repo *LevelDbRepository) GetBlockHash(blockNumber string) (string, error) {
	blockNumberBytes := common.FromHex(blockNumber)
	value, err := repo.db.Get(numberKey(blockNumberBytes), nil)
	if err != nil {
		return "", err
	}
	return common.Bytes2Hex(value), nil
}

// GetNftOwnership implements Repository.
func (repo *LevelDbRepository) GetNftOwnership(nftAddessHex string, tokenIdHex string, blockNumberHex string) (string, error) {
	nftAddessBytes := common.FromHex(nftAddessHex)
	tokenIdBytes := common.FromHex(tokenIdHex)
	blockNumberBytes := common.FromHex(blockNumberHex)

	key := nftKey(nftAddessBytes, tokenIdBytes, blockNumberBytes)
	iter := repo.db.NewIterator(nil, nil)
	if ok := iter.Seek(key); ok && bytes.Equal(iter.Key(), key) {
		return common.Bytes2Hex(iter.Value()), nil
	}

	if ok := iter.Prev(); ok && bytes.HasPrefix(iter.Key(), nftKeyPrefix(nftAddessBytes, tokenIdBytes)) {
		return common.Bytes2Hex(iter.Value()), nil
	}

	return "", types.NewNotFoundError(fmt.Sprintf("NFT(%s/%s) not found at %s from DB", nftAddessHex, tokenIdBytes, blockNumberHex))

}

// GetRecentBlock implements Repository.
func (repo *LevelDbRepository) GetRecentBlock() (BlockData, error) {
	iter := repo.db.NewIterator(nil, nil)
	numberKeyPrefix := numberKey([]byte{})
	if ok := iter.Seek(numberKeyPrefix); ok && bytes.HasPrefix(iter.Key(), numberKeyPrefix) {
		defer iter.Release()
		blockNumber := iter.Key()[len(numberKeyPrefix):]
		blockHash := iter.Value()
		return BlockData{
			Number: blockNumber,
			Hash:   blockHash,
		}, nil
	}

	return BlockData{}, errors.New("not found")
}

// PutBlockData implements Repository.
func (repo *LevelDbRepository) PutBlockData(hash []byte, number []byte, ownershipData []*types.OwnershipTransferEvent) error {
	batch := new(leveldb.Batch)
	defer batch.Reset()

	batch.Put(hashKey(hash), number)
	batch.Put(numberKey(number), hash)

	for _, v := range ownershipData {
		blockNumber := sdkmath.NewIntFromUint64(v.BlockNumber).BigInt().Bytes()
		batch.Put(nftKey(v.ContractAddr, v.TokenId.Bytes(), blockNumber), v.To)
	}

	if err := repo.db.Write(batch, nil); err != nil {
		return err
	}

	return nil
}

func hashKey(blockHash []byte) []byte {
	return append([]byte(BLOCK_HASH_PREFIX), types.PadBytes(32, blockHash)...)
}

func numberKey(blockNumber []byte) []byte {
	return append([]byte(BLOCK_NUMBER_PREFIX), types.PadBytes(32, blockNumber)...)
}

func nftKey(nftAddr []byte, tokenId []byte, blockNumber []byte) []byte {
	return append(nftKeyPrefix(nftAddr, tokenId), types.PadBytes(32, blockNumber)...)
}

func nftKeyPrefix(nftAddr []byte, tokenId []byte) []byte {
	combined := append(types.PadBytes(20, nftAddr), types.PadBytes(32, tokenId)...)
	return append([]byte(NFT_OWNERSHIP_PREFIX), combined...)
}

func (repo *LevelDbRepository) Close() {
	repo.db.Close()
}
