// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/cjdelisle/matterfoss-server/v5/model"
	"github.com/cjdelisle/matterfoss-server/v5/store"
)

type SqlSystemStore struct {
	SqlStore
}

func newSqlSystemStore(sqlStore SqlStore) store.SystemStore {
	s := &SqlSystemStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.System{}, "Systems").SetKeys(false, "Name")
		table.ColMap("Name").SetMaxSize(64)
		table.ColMap("Value").SetMaxSize(1024)
	}

	return s
}

func (s SqlSystemStore) createIndexesIfNotExists() {
}

func (s SqlSystemStore) Save(system *model.System) *model.AppError {
	if err := s.GetMaster().Insert(system); err != nil {
		return model.NewAppError("SqlSystemStore.Save", "store.sql_system.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (s SqlSystemStore) SaveOrUpdate(system *model.System) *model.AppError {
	if err := s.GetMaster().SelectOne(&model.System{}, "SELECT * FROM Systems WHERE Name = :Name", map[string]interface{}{"Name": system.Name}); err == nil {
		if _, err := s.GetMaster().Update(system); err != nil {
			return model.NewAppError("SqlSystemStore.SaveOrUpdate", "store.sql_system.update.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	} else {
		if err := s.GetMaster().Insert(system); err != nil {
			return model.NewAppError("SqlSystemStore.SaveOrUpdate", "store.sql_system.save.app_error", nil, err.Error(), http.StatusInternalServerError)
		}
	}
	return nil
}

func (s SqlSystemStore) Update(system *model.System) *model.AppError {
	if _, err := s.GetMaster().Update(system); err != nil {
		return model.NewAppError("SqlSystemStore.Update", "store.sql_system.update.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

func (s SqlSystemStore) Get() (model.StringMap, *model.AppError) {
	var systems []model.System
	props := make(model.StringMap)
	if _, err := s.GetReplica().Select(&systems, "SELECT * FROM Systems"); err != nil {
		return nil, model.NewAppError("SqlSystemStore.Get", "store.sql_system.get.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	for _, prop := range systems {
		props[prop.Name] = prop.Value
	}

	return props, nil
}

func (s SqlSystemStore) GetByName(name string) (*model.System, *model.AppError) {
	var system model.System
	if err := s.GetMaster().SelectOne(&system, "SELECT * FROM Systems WHERE Name = :Name", map[string]interface{}{"Name": name}); err != nil {
		return nil, model.NewAppError("SqlSystemStore.GetByName", "store.sql_system.get_by_name.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return &system, nil
}

func (s SqlSystemStore) PermanentDeleteByName(name string) (*model.System, *model.AppError) {
	var system model.System
	if _, err := s.GetMaster().Exec("DELETE FROM Systems WHERE Name = :Name", map[string]interface{}{"Name": name}); err != nil {
		return nil, model.NewAppError("SqlSystemStore.PermanentDeleteByName", "store.sql_system.permanent_delete_by_name.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return &system, nil
}

// InsertIfExists inserts a given system value if it does not already exist. If a value
// already exists, it returns the old one, else returns the new one.
func (s SqlSystemStore) InsertIfExists(system *model.System) (*model.System, *model.AppError) {
	tx, err := s.GetMaster().BeginTx(context.Background(), &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return nil, model.NewAppError("SqlSystemStore.InsertIfExists", "store.sql_system.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	defer finalizeTransaction(tx)

	var origSystem model.System
	if err := tx.SelectOne(&origSystem, `SELECT * FROM Systems
		WHERE Name = :Name`,
		map[string]interface{}{"Name": system.Name}); err != nil && err != sql.ErrNoRows {
		return nil, model.NewAppError("SqlSystemStore.InsertIfExists", "store.sql_system.get_by_name.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if origSystem.Value != "" {
		// Already a value exists, return that.
		return &origSystem, nil
	}

	// Key does not exist, need to insert.
	if err := tx.Insert(system); err != nil {
		return nil, model.NewAppError("SqlSystemStore.InsertIfExists", "store.sql_system.save.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	if err := tx.Commit(); err != nil {
		return nil, model.NewAppError("SqlSystemStore.InsertIfExists", "store.sql_system.save.commit_transaction.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return system, nil
}
