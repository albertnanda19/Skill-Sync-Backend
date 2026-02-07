package controller

import "strings"

func normalizeKeyword(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	keyword = strings.ToLower(keyword)
	keyword = strings.Join(strings.Fields(keyword), " ")
	return keyword
}

func lockKey(keyword string) string {
	return "scrape:lock:" + keyword
}

func lastKey(keyword string) string {
	return "scrape:last:" + keyword
}

func taskKey(keyword string) string {
	return "scrape:task:" + keyword
}

func invalidateChannel(keyword string) string {
	return "jobs:invalidate:" + keyword
}
