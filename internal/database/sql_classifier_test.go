package database

import "testing"

func TestClassifyStatementShouldRecognizeKinds(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		kind     StatementKind
		keyword  string
		hasError bool
	}{
		{name: "query", sql: "select * from demo", kind: StatementQuery, keyword: "SELECT"},
		{name: "query with comment", sql: "-- hello\nshow tables", kind: StatementQuery, keyword: "SHOW"},
		{name: "exec", sql: "insert into demo(id) values (1)", kind: StatementExec, keyword: "INSERT"},
		{name: "ddl", sql: "create table demo(id integer)", kind: StatementDDL, keyword: "CREATE"},
		{name: "multiple", sql: "select 1; select 2", hasError: true},
		{name: "unsupported", sql: "grant select on demo to app", hasError: true},
		{name: "quoted semicolon", sql: "select ';' as value", kind: StatementQuery, keyword: "SELECT"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := ClassifyStatement(tc.sql)
			if tc.hasError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info.Kind != tc.kind {
				t.Fatalf("expected kind %s, got %s", tc.kind, info.Kind)
			}
			if info.Keyword != tc.keyword {
				t.Fatalf("expected keyword %s, got %s", tc.keyword, info.Keyword)
			}
		})
	}
}
