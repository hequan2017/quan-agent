package remote

import "testing"

func TestSkillsAreBoundedAndHidden(t *testing.T) {
	listed := ListSkills()
	if len(listed) < 7 {
		t.Fatalf("expected at least 7 skills, got %d", len(listed))
	}
	for _, skill := range listed {
		if skill.ID == "" || skill.TimeoutSec <= 0 || skill.TimeoutSec > 60 {
			t.Errorf("invalid skill metadata: %#v", skill)
		}
		if skill.Command != "" {
			t.Errorf("ListSkills exposed command for %s", skill.ID)
		}
		internal, ok := getSkill(skill.ID)
		if !ok || internal.Command == "" {
			t.Errorf("skill %s has no internal command", skill.ID)
		}
	}
}
