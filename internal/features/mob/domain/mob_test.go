package domain

import "testing"

func TestMob_IsMVP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mob  Mob
		want bool
	}{
		{name: "no mvp signal", mob: Mob{}, want: false},
		{name: "mvp mode flag", mob: func() Mob {
			var modes ModeSet
			modes.Set(ModeMvp)

			return Mob{Modes: modes}
		}(), want: true},
		{name: "mvp exp implies mvp", mob: Mob{MvpExp: 1}, want: true},
		{name: "non-mvp mode is not mvp", mob: func() Mob {
			var modes ModeSet
			modes.Set(ModeAggressive)

			return Mob{Modes: modes}
		}(), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.mob.IsMVP(); got != tt.want {
				t.Errorf("IsMVP() = %v, want %v", got, tt.want)
			}
		})
	}
}
