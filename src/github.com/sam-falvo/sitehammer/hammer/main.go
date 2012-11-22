/*
The hammer command is used to process files and subdirectories in a source directory (presently assumed to be the current directory) to produce static HTML output in an output directory (presently hardwired to be ./_site).
*/
package main

import (
	"github.com/sam-falvo/sitehammer/directory"
	"io/ioutil"
	"os"
)

// outputNameFor computes a filename in the output directory which corresponds to the given input filename.
// The input filename must have a relative pathname for this to work.
// BUG(sam-falvo): Eventually, this procedure should work with absolute paths as well.
func outputNameFor(fn string) string {
	return "_site/"+fn;
}

// processSourceFile accepts a file specified by an os.FileInfo interface.
// If the file's name begins with an underscore, the file is skipped.
// Otherwise, the file is read into memory, processed, and written back out into the corresponding location in the output directory.
// Returns either an error or nil, the latter indicating a successful operation.
func processSourceFile(e os.FileInfo) error {
	inputName := e.Name();
	if inputName[0] == '_' { return nil; }
	outputName := outputNameFor(inputName);
	rawData, err := ioutil.ReadFile(inputName);
	if err != nil { return err; }
	return ioutil.WriteFile(outputName, rawData, e.Mode());
}

func main() {
	err := directory.ForEachEntry(".", func(e os.FileInfo) error {
		return directory.OnlyFiles(e, processSourceFile);
	})

	if err != nil {
		panic(err);
	}
}
