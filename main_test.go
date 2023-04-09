package main

import (
	"testing"

	_ "github.com/lib/pq"
)

/*
TODO: test for empty rows, empty columns, uneven rows
test for sql keywords in header and/or rows (e.g. "DESC")
*/

func Test_toPostgreSQLName(t *testing.T) {
	type args struct {
		s string
	}
	want1 := "lodes_2_15_2023_price_list_excel_version_vc"
	//want1 := "lodes_2_15_20" + "23_price_list_excel_version_vc"
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "CSV filename",
			args: args{
				s: "LODES 2.15. 2023 PRICE LIST -EXCEL VERSION vc.csv",
			},
			want: want1,
		},
		{
			name: "Long string",
			args: args{
				s: "LODES 2.15. 2023 PRICE LIST -EXCEL VERSION vc678901234567890123.csv",
			},
			want: "lodes_2_15_2023_price_list_excel_version_vc6789012345678901",
		},
		{
			name: "Non-breaking space",
			args: args{
				s: "LODES 2.15. 2023" + string('\u00A0') + string('\u00A0') + "PRICE LIST -EXCEL VERSION vc.csv",
			},
			want: want1,
		},
		{
			name: "Initial number",
			args: args{
				s: "99_LODES 2.15. 2023 PRICE LIST -EXCEL VERSION vc.csv",
			},
			want: want1,
		},
		{
			name: "Initial underscore",
			args: args{
				s: "_99_LODES 2.15. 2023 PRICE LIST -EXCEL VERSION vc.csv",
			},
			want: want1,
		},
		{
			name: "Multiple initial disallowed chars",
			args: args{
				s: "*_99 99 LODES 2.15. 2023 PRICE LIST -EXCEL VERSION vc.csv",
			},
			want: want1,
		},
		{
			name: "Non-ascii",
			args: args{
				s: "_99_LODÉS 2.15. 2023 PRIČÈ LIST -EXCEL VERSION vcЮ.csv",
			},
			want: want1,
		},
		{
			name: "Initial BOM",
			args: args{
				s: string('\uFEFF') + "LODES 2.15. 2023 PRICE LIST -EXCEL VERSION vc.csv",
			},
			want: want1,
		},
		{
			name: "Initial BOM and non-ascii",
			args: args{
				s: string('\uFEFF') + "_99_LODÉS 2.15. 2023 PRIČÈ LIST -EXCEL VERSION vcЮ.csv",
			},
			want: want1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toPostgreSQLName(tt.args.s); got != tt.want {
				t.Errorf("toPostgreSQLName() = %v, want %v", got, tt.want)
			}
		})
	}
}

/* //go:embed evolutions/*.sql
var evolutionFS embed.FS

// Ensure the test database can open & close.
func TestDB(t *testing.T) {
	db := MustOpenDB(t)
	MustCloseDB(t, db)
}

// MustOpenDB returns a new, open DB. Fatal on error.
func MustOpenDB(tb testing.TB) *sqlite.DB {
	tb.Helper()

	// Write to an in-memory database by default.
	// If the -dump flag is set, generate a temp file for the database.
	//dsn := ":memory:"
	dsn := "file:test.db?cache=shared&mode=rwc&locking_mode=NORMAL&_fk=1&synchronous=2"
	if *dump {
		dir, err := ioutil.TempDir("", "")
		if err != nil {
			tb.Fatal(err)
		}
		dsn = filepath.Join(dir, "db") // TODO: this is dumb. Tosses out my entire DSN
		println("DUMP=" + dsn)
	}

	db := sqlite.NewDB(dsn)
	if err := db.Open(); err != nil {
		tb.Fatal(err)
	}
	return db
}

// MustCloseDB closes the DB. Fatal on error.
func MustCloseDB(tb testing.TB, db *sqlite.DB) {
	tb.Helper()
	if err := db.Close(); err != nil {
		tb.Fatal(err)
	}
}

func TestCreateTableForCSV(t *testing.T) {
	testCases := []struct {
		name         string
		csv          string
		expectedCols []string
	}{
		{
			name: "Simple CSV",
			csv: `name,age,email
			Jane,30,jane@example.com
			John,40,john@example.com`,
			expectedCols: []string{"name", "age", "email"},
		},
		{
			name: "Empty CSV",
			csv:  ``,
		},
		{
			name: "CSV with empty columns",
			csv: `name,,email
			Jane,,jane@example.com
			John,,john@example.com`,
			expectedCols: []string{"name", "email"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file := strings.NewReader(tc.csv)
			tx, err := db.Begin()
			if err != nil {
				t.Fatalf("failed to begin transaction: %v", err)
			}
			defer tx.Rollback()

			columns, err := createTableForCSV(tx, file, "test_table")
			if err != nil {
				t.Fatalf("createTableForCSV failed: %v", err)
			}

			// Check the column names
			if len(columns) != len(tc.expectedCols) {
				t.Errorf("unexpected column count; got %d, want %d", len(columns), len(tc.expectedCols))
			}
			for i, col := range columns {
				if col != tc.expectedCols[i] {
					t.Errorf("unexpected column name at index %d; got %s, want %s", i, col, tc.expectedCols[i])
				}
			}

			// Check the table schema
			schemaRows, err := tx.Query("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'test_table'")
			if err != nil {
				t.Fatalf("failed to query table schema: %v", err)
			}
			defer schemaRows.Close()

			schemaCols := []struct {
				name string
				typ  string
			}{}
			for schemaRows.Next() {
				var colName, colType string
				if err := schemaRows.Scan(&colName, &colType); err != nil {
					t.Fatalf("failed to scan table schema row: %v", err)
				}
				schemaCols = append(schemaCols, struct {
					name string
					typ  string
				}{name: colName, typ: colType})
			}
			if err := schemaRows.Err(); err != nil {
				t.Fatalf("failed to read table schema rows: %v", err)
			}

			if len(schemaCols) != len(tc.expectedCols)+1 {
				t.Errorf("unexpected schema column count; got %d, want %d", len(schemaCols), len(tc.expectedCols)+1)
			}
			if schemaCols[0].name != "_id" || schemaCols[0].typ != "integer" {
				t.Errorf("unexpected system column; got (%s, %s), want (_id, integer)", schemaCols[0].name, schemaCols[0].typ)
			}
			for i, expectedCol := range tc.expectedCols {
				actualCol := schemaCols[i+1]
				if actualCol.name != expectedCol {
					t.Errorf("unexpected column name at index %d; got %s, want %s", i, actualCol.name, expectedCol)
				}
				if actualCol.typ != "character varying" {
					t.Errorf("unexpected column type at index %d; got %s, want character varying", i, actualCol.typ)
				}
			}
		})
	}
} */
