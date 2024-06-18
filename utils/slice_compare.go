package utils

// IdCompare
// Change Increase Decrease
func IdCompare(ids1, ids2 []uint) (change []uint, increase []uint, decrease []uint) {
	m := make(map[uint]bool)
	for _, item := range ids1 {
		m[item] = true
	}

	sameMap := make(map[uint]bool)
	for _, id := range ids2 {
		_, ok := m[id]
		if ok {
			//changed
			change = append(change, id)
			sameMap[id] = true
		} else {
			//add
			increase = append(increase, id)
		}
	}

	//decrease
	for _, id := range ids1 {
		_, ok := sameMap[id]
		if !ok {
			decrease = append(decrease, id)
		}
	}

	return change, increase, decrease
}
