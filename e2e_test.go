// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alloydbconn_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/alloydbconn"
	"github.com/jackc/pgx/v4"
)

var (
	// AlloyDB instance name, in the form of
	// projects/PROJECT_ID/locations/REGION_ID/clusters/CLUSTER_ID/instances/INSTANCE_ID
	alloydbInstanceName = os.Getenv("ALLOYDB_INSTANCE_NAME")
	// Name of database user.
	alloydbUser = os.Getenv("ALLOYDB_USER")
	// Name of database IAM user.
	alloydbIAMUser = os.Getenv("ALLOYDB_IAM_USER")
	// IP address of the instance
	alloydbInstanceIP = os.Getenv("ALLOYDB_INSTANCE_IP")
	// Password for the database user; be careful when entering a password on the
	// command line (it may go into your terminal's history).
	alloydbPass = os.Getenv("ALLOYDB_PASS")
	// Name of the database to connect to.
	alloydbDB = os.Getenv("ALLOYDB_DB")
)

func requireAlloyDBVars(t *testing.T) {
	switch "" {
	case alloydbInstanceName:
		t.Fatal("'ALLOYDB_INSTANCE_NAME' env var not set")
	case alloydbUser:
		t.Fatal("'ALLOYDB_USER' env var not set")
	case alloydbIAMUser:
		t.Fatal("'ALLOYDB_IAM_USER' env var not set")
	case alloydbPass:
		t.Fatal("'ALLOYDB_PASS' env var not set")
	case alloydbDB:
		t.Fatal("'ALLOYDB_DB' env var not set")
	}
}

func TestPgxConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}
	requireAlloyDBVars(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, cleanup, err := connectPgx(
		ctx, alloydbInstanceName, alloydbUser, alloydbPass, alloydbDB,
	)
	if err != nil {
		_ = cleanup()
		t.Fatal(err)
	}
	defer func() {
		pool.Close()
		// best effort
		_ = cleanup()
	}()

	var now time.Time
	err = pool.QueryRow(context.Background(), "SELECT NOW()").Scan(&now)
	if err != nil {
		t.Fatalf("QueryRow failed: %s", err)
	}
	t.Log(now)
}

func TestDatabaseSQLConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration tests")
	}

	pool, cleanup, err := connectDatabaseSQL(
		alloydbInstanceName, alloydbUser, alloydbPass, alloydbDB,
	)
	if err != nil {
		_ = cleanup()
		t.Fatal(err)
	}
	defer func() {
		pool.Close()
		// best effort
		_ = cleanup()
	}()
}

func TestDirectDatabaseSQLAutoIAMAuthN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, err := connectDirectDatabaseSQLAutoIAMAuthN(
		alloydbInstanceIP, alloydbIAMUser, alloydbDB,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var tt time.Time
	if err := db.QueryRow("SELECT NOW()").Scan(&tt); err != nil {
		t.Fatal(err)
	}
	t.Log(tt)
}

func TestDirectPGXAutoIAMAuthN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	db, err := connectDirectPGXPoolAutoIAMAuthN(
		context.Background(),
		alloydbInstanceIP, alloydbIAMUser, alloydbDB,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var tt time.Time
	if err := db.QueryRow(context.Background(), "SELECT NOW()").Scan(&tt); err != nil {
		t.Fatal(err)
	}
	t.Log(tt)
}

func TestAutoIAMAuthN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	ctx := context.Background()

	d, err := alloydbconn.NewDialer(ctx, alloydbconn.WithIAMAuthN())
	if err != nil {
		t.Fatalf("failed to init Dialer: %v", err)
	}

	dsn := fmt.Sprintf(
		"user=%s dbname=%s sslmode=disable",
		alloydbIAMUser, alloydbDB,
	)
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("failed to parse pgx config: %v", err)
	}

	config.DialFunc = func(ctx context.Context, network string, instance string) (net.Conn, error) {
		return d.Dial(ctx, alloydbInstanceName)
	}

	conn, connErr := pgx.ConnectConfig(ctx, config)
	if connErr != nil {
		t.Fatalf("failed to connect: %s", connErr)
	}
	defer conn.Close(ctx)

	var tt time.Time
	if err := conn.QueryRow(context.Background(), "SELECT NOW()").Scan(&tt); err != nil {
		t.Fatal(err)
	}
	t.Log(tt)
}
