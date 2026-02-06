package seeder

func Defaults() []Seeder {
	return []Seeder{
		SkillsSeeder{},
		JobSourcesSeeder{},
	}
}
