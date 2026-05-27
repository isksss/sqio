package dbdriver

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		DriverSQLite:      DriverSQLite,
		DriverSQLite3:     DriverSQLite,
		DriverDuckDB:      DriverDuckDB,
		DriverPostgres:    DriverPGX,
		DriverPostgreSQL:  DriverPGX,
		DriverPGX:         DriverPGX,
		DriverCockroach:   DriverPGX,
		DriverCockroachDB: DriverPGX,
		DriverMySQL:       DriverMySQL,
		DriverMariaDB:     DriverMySQL,
		DriverTiDB:        DriverMySQL,
		DriverSQLServer:   DriverSQLServer,
		DriverMSSQL:       DriverSQLServer,
		DriverOracle:      DriverOracle,
		DriverClickHouse:  DriverClickHouse,
		DriverCH:          DriverClickHouse,
	}
	for driver, want := range tests {
		t.Run(driver, func(t *testing.T) {
			got, ok := Normalize(driver)
			if !ok || got != want {
				t.Fatalf("Normalize(%q) = %q, %v; want %q, true", driver, got, ok, want)
			}
		})
	}
}

func TestClassificationAndDefaultPort(t *testing.T) {
	if !IsPostgresFamily(DriverPostgreSQL) || DefaultPort(DriverPostgres) != 5432 {
		t.Fatal("postgres alias classification failed")
	}
	if !IsMySQLFamily(DriverMariaDB) || DefaultPort(DriverTiDB) != 3306 {
		t.Fatal("mysql alias classification failed")
	}
	if !IsSQLServer(DriverMSSQL) || DefaultPort(DriverMSSQL) != 1433 {
		t.Fatal("sqlserver alias classification failed")
	}
	if !IsClickHouse(DriverCH) || DefaultPort(DriverCH) != 9000 {
		t.Fatal("clickhouse alias classification failed")
	}
	if IsSQLite("unknown") || Supported("unknown") {
		t.Fatal("unknown driver should not be supported")
	}
}
