package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

func main() {
	fmt.Println("hey this is from inside the container")
	err := filepath.Walk("/", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		/*if info.IsDir() && info.Name() == subDirToSkip {
			fmt.Printf("skipping a dir without errors: %+v \n", info.Name())
			return filepath.SkipDir
		}*/
		fmt.Printf("visited file or dir: %q\n", path)
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path: %v\n", err)
		return
	}
}
