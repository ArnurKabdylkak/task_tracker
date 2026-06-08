package config

import "testing"

// BOSS_IDS принимает вперемешку числовые ID и @username; IsBoss срабатывает
// по любому совпадению, username сравнивается без учёта регистра и префикса @.
func TestIsBoss(t *testing.T) {
	t.Setenv("BOT_TOKEN", "x")
	t.Setenv("BOSS_IDS", "123, @Ivan, olga")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		id       int64
		username string
		want     bool
	}{
		{"по ID", 123, "", true},
		{"по нику с @ в конфиге, без @ у юзера", 1, "ivan", true},
		{"ник в другом регистре", 1, "IVAN", true},
		{"ник без @ в конфиге", 2, "Olga", true},
		{"чужой ID и ник", 999, "petr", false},
		{"пустой ник, чужой ID", 999, "", false},
	}
	for _, c := range cases {
		if got := cfg.IsBoss(c.id, c.username); got != c.want {
			t.Errorf("%s: IsBoss(%d, %q) = %v, ожидалось %v", c.name, c.id, c.username, got, c.want)
		}
	}
}
