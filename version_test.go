package hypro

import "testing"

func Test_checkVersionCompatible(t *testing.T) {
	type args struct {
		clientVersion string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"Same version", args{Version}, true, false},
		{"Older version", args{"0.0.1"}, false, false},
		{"Newer version", args{"999.999.999"}, false, false},
		{"Invalid version", args{"v1.0.0"}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkVersionCompatible(tt.args.clientVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkVersionCompatible() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("checkVersionCompatible() = %v, want %v", got, tt.want)
			}
		})
	}
}
