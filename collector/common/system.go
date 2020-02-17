package common

import "strings"

const (
	adminDB  = "admin"
	configDB = "config"
	localDB  = "local"
)

const sysCollPrefix = "system."

// IsSystemDB tests whether system db name passed
func IsSystemDB(dbName string) bool {
	switch dbName {
	case adminDB, configDB, localDB:
		return true
	default:
		return false
	}
}

// IsSystemCollection tests whether system collection name passed
func IsSystemCollection(collName string) bool {
	return strings.HasPrefix(collName, sysCollPrefix)
}
