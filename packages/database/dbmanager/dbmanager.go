package dbmanager

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/timeutil"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/parameters"
)

type ChainKVStoreProvider func(chainID *iscp.ChainID) kvstore.KVStore

type DBManager struct {
	log           *logger.Logger
	registryDB    database.DB
	registryStore kvstore.KVStore
	databases     map[[ledgerstate.AddressLength]byte]database.DB
	stores        map[[ledgerstate.AddressLength]byte]kvstore.KVStore
	mutex         sync.RWMutex
	inMemory      bool
}

func NewDBManager(log *logger.Logger, inMemory bool) *DBManager {
	dbm := DBManager{
		log:       log,
		databases: make(map[[ledgerstate.AddressLength]byte]database.DB),
		stores:    make(map[[ledgerstate.AddressLength]byte]kvstore.KVStore),
		mutex:     sync.RWMutex{},
		inMemory:  inMemory,
	}
	// registry db is created with an empty chainID
	dbm.registryDB = dbm.createDB(nil)
	dbm.registryStore = dbm.registryDB.NewStore()
	return &dbm
}

func getChainBase58(chainID *iscp.ChainID) string {
	if chainID != nil {
		return chainID.Base58()
	}
	return "CHAIN_REGISTRY"
}

func (m *DBManager) createDB(chainID *iscp.ChainID) database.DB {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if chainID == nil {
		m.log.Infof("creating new registry database. Persistent: %v", !m.inMemory)
	} else {
		m.log.Infof("creating new database for chain %s. Persistent: %v", chainID.String(), !m.inMemory)
	}
	if m.inMemory {
		db, err := database.NewMemDB()
		if err != nil {
			m.log.Fatal(err)
		}
		return db
	}

	dbDir := parameters.GetString(parameters.DatabaseDir)
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		// create a new database dir if none exists
		err := os.Mkdir(dbDir, os.ModePerm)
		if err != nil {
			m.log.Fatal(err)
			return nil
		}
	}
	instanceDir := fmt.Sprintf("%s/%s", dbDir, getChainBase58(chainID))
	db, err := database.NewDB(instanceDir)
	if err != nil {
		m.log.Fatal(err)
	}
	return db
}

func (m *DBManager) GetRegistryKVStore() kvstore.KVStore {
	return m.registryStore
}

func (m *DBManager) GetOrCreateKVStore(chainID *iscp.ChainID) kvstore.KVStore {
	store := m.GetKVStore(chainID)
	if store != nil {
		return store
	}

	// create a new database / store
	db := m.createDB(chainID)
	store = db.NewStore()
	m.databases[chainID.Array()] = db
	m.stores[chainID.Array()] = db.NewStore()
	return store
}

func (m *DBManager) GetKVStore(chainID *iscp.ChainID) kvstore.KVStore {
	return m.stores[chainID.Array()]
}

func (m *DBManager) Close() {
	m.registryDB.Close()
	for _, instance := range m.databases {
		instance.Close()
	}
}

func (m *DBManager) RunGC(shutdownSignal <-chan struct{}) {
	m.gc(m.registryDB, shutdownSignal)
	for _, db := range m.databases {
		m.gc(db, shutdownSignal)
	}
}

func (m *DBManager) gc(db database.DB, shutdownSignal <-chan struct{}) {
	if !db.RequiresGC() {
		return
	}
	// run the garbage collection with the given interval
	gcTimeInterval := 5 * time.Minute
	timeutil.NewTicker(func() {
		if err := db.GC(); err != nil {
			m.log.Warnf("Garbage collection failed: %s", err)
		}
	}, gcTimeInterval, shutdownSignal)
}
