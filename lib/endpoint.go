package lib

import (
	"github.com/donghquinn/gdct"
	"org.donghyuns.com/rtsphls/configs"
)

func GetDataUrl(cctvId string) (string, error) {
	var streamUrl string

	dbConfig := configs.DatabaseConfig
	sslMode := "disable"

	conn, connErr := gdct.InitPostgresConnection(gdct.DBConfig{
		Host:     dbConfig.Host,
		Port:     dbConfig.Port,
		UserName: dbConfig.User,
		Password: dbConfig.Passwd,
		Database: dbConfig.Database,
		SslMode:  &sslMode,
	})

	if connErr != nil {
		return streamUrl, connErr
	}

	queryResult, queryErr := conn.PgSelectSingle("SELECT streaming1_adres FROM m_fa_cctv WHERE cctv_id = $1", cctvId)

	if queryErr != nil {
		return streamUrl, queryErr
	}

	if scanErr := queryResult.Scan(&streamUrl); scanErr != nil {
		return streamUrl, scanErr
	}

	return streamUrl, nil
}
