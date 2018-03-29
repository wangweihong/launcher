package cluster

func imageArray2Map(array []Image) map[string]string{
	res := make(map[string]string)

	for i := range array {
		if len(array[i].Id) > 0 && len(array[i].Path) > 0 {
			res[array[i].Id] = array[i].Path
		}
	}

	return res
}
