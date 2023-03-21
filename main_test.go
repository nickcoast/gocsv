package main

import (
	"testing"

	_ "github.com/lib/pq"
)

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
