package main

import (
	"archive/zip"
	"compress/flate"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
)

// We want to stream the payloads out to the resulting zip file as we get them so we
// don't stall for too long.
type payload struct {
	Name string
	Data []byte
}

func main() {
	fmt.Println("zip-example")
	ZipWriter(fmt.Sprintf("hello-%d", time.Now().Unix()))
}

func ZipWriter(name string) {
	ctx := context.TODO()
	outFile, err := os.Create(fmt.Sprintf("%s.zip", name))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer outFile.Close()

	// Create a new zip archive.
	z := zip.NewWriter(outFile)

	// Crank up the compression
	z.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	defer z.Close()
	payloadsChan := make(chan payload, 10) // Buffer a little bit to smooth it out
	doneChan := make(chan error)
	go func() {
		for p := range payloadsChan {
			h := &zip.FileHeader{Name: p.Name, Method: zip.Deflate}
			h.SetModTime(time.Now())
			f, err := z.CreateHeader(h)
			if err != nil {
				msg := fmt.Sprintf("Failed to create %s in support dump: %s", p.Name, err)
				log.Error(msg)
				doneChan <- fmt.Errorf(msg)
				return
			}
			log.Debugf("Support dump: writing %s", p.Name)
			_, err = f.Write(p.Data)
			if err != nil {
				msg := fmt.Sprintf("Failed to write payload %s in support dump: %s", p.Name, err)
				log.Error(msg)
				doneChan <- fmt.Errorf(msg)
				return
			}
			z.Flush() // Try to keep the client stream going without buffering too long
		}
		doneChan <- nil
		log.Debugf("Streaming complete")
	}()
	for configName, data := range getInterlockConfigs(ctx) {
		payloadsChan <- payload{Name: configName, Data: data}
	}
	close(payloadsChan)
	log.Debugf("Waiting for stream to finish")

	err = <-doneChan
	log.Debugf("Complete")
}

func getInterlockConfigs(ctx context.Context) map[string][]byte {
	return map[string][]byte{
		"configs/interlock-metadata.txt": []byte("metdata"),
		"configs/interlock-conf-1.txt":   []byte("conf1"),
		"configs/interlock-conf-2.txt":   []byte("conf2"),
	}
}

func addFiles(w *zip.Writer, basePath, baseInZip string) {
	// Open the Directory
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		fmt.Println(err)
	}

	for _, file := range files {
		fmt.Println(basePath + file.Name())
		if !file.IsDir() {
			dat, err := ioutil.ReadFile(basePath + file.Name())
			if err != nil {
				fmt.Println(err)
			}

			// Add some files to the archive.
			f, err := w.Create(baseInZip + file.Name())
			if err != nil {
				fmt.Println(err)
			}
			_, err = f.Write(dat)
			if err != nil {
				fmt.Println(err)
			}
		} else if file.IsDir() {

			// Recurse
			newBase := basePath + "/" + file.Name() + "/"
			fmt.Println("Recursing and Adding SubDir: " + file.Name())
			fmt.Println("Recursing and Adding SubDir: " + newBase)

			addFiles(w, newBase, file.Name()+"/")
		}
	}
}
