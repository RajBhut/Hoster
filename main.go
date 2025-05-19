package main

import (
	"fmt"
)

// func main() {
// 	reader := bufio.NewReader(os.Stdin)
// 	fmt.Println("Enter Your name: ")
// 	name, _ := reader.ReadString('\n')
// 	name = strings.TrimSpace(name)

//	greet(name)
//
// fmt.Println("hey buddy whats your age? ")
//
//	var age int
//	fmt.Scanln(&age)
//	age_checker(age)
//
// say_hellow(5)
// array
// var arr [3]int
// arr[1] = 1
// fmt.Println(arr)
// var sl []int
// sl = append(sl, 2)
// fmt.Println(sl)
// slice_sum()
// students()
//
//		bio()
//	}

func age_checker(age int) {
	if age >= 60 {
		fmt.Println("You are Senior ")
	} else if age > 18 {
		fmt.Println("You are adult")
	} else {
		fmt.Println("You are Kid")
	}
}
func test() {

	//scanln just read till \n or space
	var name string
	fmt.Scanln(&name)
	fmt.Println(name)
}

func slice_sum() {
	fmt.Println("Enter numbers want to add")
	var num int
	var sum int
	var arr []int
	fmt.Scanln(&num)
	for i := range num {
		fmt.Println("Enter your ", i+1, "st number : ")
		var tem int
		fmt.Scanln(&tem)
		arr = append(arr, tem)

	}
	for _, value := range arr {
		sum += value
	}

	fmt.Println("Your sum is : ", sum)
}
func students() {
	fmt.Println("Enter number of student want to add")
	var num int
	students := map[string]int{}

	fmt.Scanln(&num)
	for range num {
		fmt.Println("enter  name: ")
		var name string
		fmt.Scanln(&name)
		fmt.Println("enter marks : ")
		var ts int
		fmt.Scanln(&ts)
		students[name] = ts

	}
	sum := 0
	for value := range students {
		tem := students[value]
		fmt.Println(value, " : ", tem)
		sum += students[value]
	}
	fmt.Println("avrage is: ", sum/len(students))
}

type data struct {
	name   string
	age    int
	number string
}

func bio() {
	d1 := data{"User1", 20, "no one"}
	fmt.Print(d1)
}
