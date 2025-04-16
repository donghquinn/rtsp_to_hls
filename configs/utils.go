package configs

import (
	"os"
	"strconv"
)

// GetEnv retrieves an environment variable and indicates if it exists
func GetEnv(key string) (string, bool) {
	val, exists := os.LookupEnv(key)
	return val, exists
}

// GetEnvOrDefault retrieves an environment variable or returns a default value
func GetEnvOrDefault(key, defaultValue string) string {
	if val, exists := GetEnv(key); exists && val != "" {
		return val
	}
	return defaultValue
}

// GetEnvAsInt retrieves an environment variable as an integer
func GetEnvAsInt(key string, defaultValue int) int {
	if val, exists := GetEnv(key); exists && val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// GetEnvAsBool retrieves an environment variable as a boolean
func GetEnvAsBool(key string, defaultValue bool) bool {
	if val, exists := GetEnv(key); exists && val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultValue
}
