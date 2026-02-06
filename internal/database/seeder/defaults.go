package seeder

import appseeder "skill-sync/internal/seeder"

func Defaults() []Seeder {
	return []Seeder{
		SkillsSeeder{},
		JobSourcesSeeder{},
		appseeder.JobSeeder{},
		appseeder.JobRequiredSkillSeeder{},
	}
}
