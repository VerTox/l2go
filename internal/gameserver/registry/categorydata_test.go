package registry

import "testing"

func TestCategoryDataInCategory(t *testing.T) {
	c := NewCategoryData()
	if err := c.load([]byte(`<list>
<category name="HUMAN_FALL_CLASS"><id>0</id><id>1</id></category>
</list>`)); err != nil {
		t.Fatal(err)
	}
	if !c.InCategory("HUMAN_FALL_CLASS", 0) {
		t.Fatal("class 0 must be in HUMAN_FALL_CLASS")
	}
	if c.InCategory("HUMAN_FALL_CLASS", 99) {
		t.Fatal("class 99 must not be in HUMAN_FALL_CLASS")
	}
	if c.InCategory("NO_SUCH_CATEGORY", 0) {
		t.Fatal("unknown category must be false")
	}
}
