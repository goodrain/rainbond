/*
Package pinyin : 汉语拼音转换工具.

Usage

	package main

	import (
		"fmt"
		"github.com/mozillazg/go-pinyin"
	)

	func main() {
		hans := "中国人"
		// 默认
		a := pinyin.NewArgs()
		fmt.Println(pinyin.Pinyin(hans, a))
		// [[zhong] [guo] [ren]]

		// 包含声调
		a.Style = pinyin.Tone
		fmt.Println(pinyin.Pinyin(hans, a))
		// [[zhōng] [guó] [rén]]

		// 声调用数字表示
		a.Style = pinyin.Tone2
		fmt.Println(pinyin.Pinyin(hans, a))
		// [[zho1ng] [guo2] [re2n]]

		// 开启多音字模式
		a = pinyin.NewArgs()
		a.Heteronym = true
		fmt.Println(pinyin.Pinyin(hans, a))
		// [[zhong zhong] [guo] [ren]]
		a.Style = pinyin.Tone2
		fmt.Println(pinyin.Pinyin(hans, a))
		// [[zho1ng zho4ng] [guo2] [re2n]]
	}
*/
package pinyin
