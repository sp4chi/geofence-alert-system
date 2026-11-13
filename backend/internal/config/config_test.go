package config

import (
	"reflect"
	"testing"
)

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": "postgres://x"},
			want: Config{Port: "8080", DatabaseURL: "postgres://x", RestrictedZoneDefaultAlert: true},
		},
		{
			name: "all set",
			env: map[string]string{
				"PORT":                          "9000",
				"DATABASE_URL":                  "postgres://x",
				"CORS_ORIGINS":                  "http://a.com, https://b.app,,",
				"API_KEY":                       "secret",
				"RATE_LIMIT_RPS":                "5.5",
				"RESTRICTED_ZONE_DEFAULT_ALERT": "false",
			},
			want: Config{
				Port:         "9000",
				DatabaseURL:  "postgres://x",
				CORSOrigins:  []string{"http://a.com", "https://b.app"},
				APIKey:       "secret",
				RateLimitRPS: 5.5,
			},
		},
		{
			name: "origins pasted as full URLs are reduced to scheme://host",
			env: map[string]string{
				"DATABASE_URL": "postgres://x",
				"CORS_ORIGINS": `https://app.vercel.app/geofences, https://app2.vercel.app/ ,"http://localhost:5173",*,HTTPS://Upper.Example`,
			},
			want: Config{
				Port:                       "8080",
				DatabaseURL:                "postgres://x",
				CORSOrigins:                []string{"https://app.vercel.app", "https://app2.vercel.app", "http://localhost:5173", "*", "https://upper.example"},
				RestrictedZoneDefaultAlert: true,
			},
		},
		{name: "missing database url", env: map[string]string{}, wantErr: true},
		{name: "bad rate limit", env: map[string]string{"DATABASE_URL": "x", "RATE_LIMIT_RPS": "fast"}, wantErr: true},
		{name: "negative rate limit", env: map[string]string{"DATABASE_URL": "x", "RATE_LIMIT_RPS": "-1"}, wantErr: true},
		{name: "bad bool", env: map[string]string{"DATABASE_URL": "x", "RESTRICTED_ZONE_DEFAULT_ALERT": "maybe"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := load(fakeEnv(tt.env))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}
