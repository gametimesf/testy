package fib

func Fib(n int) int {
	if n < 2 {
		return n
	}

	i, j := 0, 1
	for x := 2; x <= n; x++ {
		i, j = j, j+i
	}
	return j
}
