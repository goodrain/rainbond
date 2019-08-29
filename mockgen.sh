#!/bin/bash
mockgen -source=worker/appm/store/store.go -destination=worker/appm/store/mock_store.go -package=store
mockgen -source=db/db.go -destination=db/db_mock.go -package=db
mockgen -source=db/dao/dao.go -destination=db/dao/dao_mock.go -package=dao
mockgen -source=event/manager.go -destination=event/manager_mock.go -package=event