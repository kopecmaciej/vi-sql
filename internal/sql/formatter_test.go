package sql

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "select where",
			sql:  "select id, name from users where active = true and age > 18",
			want: "select id, name\nfrom users\nwhere active = true and age > 18",
		},
		{
			name: "join chain with order and limit",
			sql:  "select u.id, o.total from users u left join orders o on o.user_id = u.id where o.total > 100 order by o.total desc limit 10",
			want: "select u.id, o.total\nfrom users u\nleft join orders o\non o.user_id = u.id\nwhere o.total > 100\norder by o.total desc\nlimit 10",
		},
		{
			name: "group by having order limit offset",
			sql:  "select id from users where active = true group by id having count(*) > 1 order by id limit 5 offset 10",
			want: "select id\nfrom users\nwhere active = true\ngroup by id\nhaving count(*) > 1\norder by id\nlimit 5\noffset 10",
		},
		{
			name: "subquery in from",
			sql:  "select id from (select user_id from orders where amount > 100) o where o.user_id is not null",
			want: "select id\nfrom (\n    select user_id\n    from orders\n    where amount > 100\n) o\nwhere o.user_id is not null",
		},
		{
			name: "subquery in where",
			sql:  "select id from users where id in (select user_id from orders) and name = 'bob'",
			want: "select id\nfrom users\nwhere id in (\n    select user_id\n    from orders\n) and name = 'bob'",
		},
		{
			name: "multiple statements",
			sql:  "select 1; select 2;",
			want: "select 1;\n\nselect 2;",
		},
		{
			name: "no trailing semicolon is not added",
			sql:  "select 1; select 2",
			want: "select 1;\n\nselect 2",
		},
		{
			name: "function call has no space before paren",
			sql:  "select count(*) from users",
			want: "select count(*)\nfrom users",
		},
		{
			name: "already formatted input is unchanged",
			sql:  "select id\nfrom users\nwhere active = true",
			want: "select id\nfrom users\nwhere active = true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Format(tt.sql)
			if got != tt.want {
				t.Errorf("Format(%q) =\n%s\nwant:\n%s", tt.sql, got, tt.want)
			}
			if again := Format(got); again != got {
				t.Errorf("Format is not idempotent for %q:\nfirst:\n%s\nsecond:\n%s", tt.sql, got, again)
			}
		})
	}
}
