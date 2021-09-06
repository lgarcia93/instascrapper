package core

func GetListTypeDescription(listType ListType) string {
	var listTypeString string = "followers"

	switch listType {
	case Followers:
		listTypeString = "followers"
	case Following:
		listTypeString = "following"
	}

	return listTypeString
}
