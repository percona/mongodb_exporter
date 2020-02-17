package common

import "strings"

const (
	adminDB  = "admin"
	configDB = "config"
	localDB  = "local"
)

const sysCollPrefix = "system."

func IsSystemDB(dbName string) bool {
	switch dbName {
	case adminDB, configDB, localDB:
		return true
	default:
		return false
	}
}

func IsSystemCollection(collName string) bool {
	return strings.HasPrefix(collName, sysCollPrefix)
}
