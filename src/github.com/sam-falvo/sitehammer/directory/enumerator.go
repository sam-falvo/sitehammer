/*
The directory package contains useful utilities relating to directories, at least as far as sitehammer is concerned.
*/
package directory

import (
	"io/ioutil"
	"os"
)

// MemberHandler functions are used by enumerators or filters to process what's found inside a directory.
// A MemberHandler takes an os.FileInfo as input, and returns either nil if processing is successful,
// or some error otherwise.  Unless specified otherwise, returning an error will terminate further enumeration.
type MemberHandler func(os.FileInfo) error;

// ForEachEntry enumerates every directory entry found by ioutil.ReadDir, calling a function
// f for each one.  See the type MemberHandler for the signature and semantics of f.
func ForEachEntry(d string, f MemberHandler) error {
	entries, err := ioutil.ReadDir(d);
	if err != nil {
		return err;
	}

	for _, entry := range entries {
		err := f(entry);
		if err != nil {
			return err;
		}
	}

	return nil;
}

// OnlyFiles should be used with ForEachEntry to filter out only files.
func OnlyFiles(inp os.FileInfo, f MemberHandler) error {
	if inp.IsDir() { return nil; }
	return f(inp);
}

