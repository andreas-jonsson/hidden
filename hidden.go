/*
Copyright (C) 2017 Andreas T Jonsson

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"hash/adler32"
	"image"
	"io"
	"io/ioutil"
	"os"
	"path"

	"golang.org/x/image/bmp"
)

func main() {
	fmt.Println("Hidden Message")
	fmt.Println("Copyright (C) 2017 Andreas T Jonsson\n")

	enc := flag.String("encode", "", "BMP image to hide message in.")
	dec := flag.String("decode", "", "Decode message in BMP image.")
	msg := flag.String("msg", "", "Message or data to encode/decode.")

	flag.Parse()
	if *msg != "" {
		if *dec != "" {
			decode(*dec, *msg)
			fmt.Println("Done!")
			return
		} else if *enc != "" {
			dest := path.Join(path.Dir(*enc), "encoded.bmp")
			encode(*enc, dest, *msg)
			fmt.Println("Done!")
			return
		}
	}

	flag.PrintDefaults()
	fatal()
}

func fatal(msg ...interface{}) {
	fmt.Println(msg...)
	os.Exit(-1)
}

func openImage(file string) *image.RGBA {
	fp, err := os.Open(file)
	if err != nil {
		fatal(err)
	}
	defer fp.Close()

	img, err := bmp.Decode(fp)
	if err != nil {
		fatal(err)
	}

	rgbaImg, ok := img.(*image.RGBA)
	if !ok {
		fatal("expected 24bpp bmp image")
	}
	return rgbaImg
}

func decode(fin, fout string) {
	var (
		img = openImage(fin)
		buf bytes.Buffer
		res byte
		j   uint32
	)

	for i, b := range img.Pix {
		if (i+1)%4 == 0 {
			continue
		}

		bit := b % 2
		res |= bit << (7 - j%8)
		j++

		if j%8 == 0 {
			buf.WriteByte(res)
			res = 0
		}
	}

	var (
		size, hash uint32
	)

	binary.Read(&buf, binary.BigEndian, &size)
	binary.Read(&buf, binary.BigEndian, &hash)

	const noHiddenMessage = "image did not contain a hidden message"

	msg := buf.Bytes()
	if len(msg) < int(size) {
		fatal(noHiddenMessage)
	}
	msg = msg[:size]

	if adler32.Checksum(msg) != hash {
		fatal(noHiddenMessage)
	}

	if err := ioutil.WriteFile(fout, msg, 0777); err != nil {
		fatal(err)
	}
}

func encode(fin, fout, fmsg string) {
	srcImg := openImage(fin)
	destImg := image.NewRGBA(srcImg.Bounds())
	r := newBitReader(fmsg)

	ln := len(srcImg.Pix)
	if len(r.data)+ln/4 > ln {
		fatal("message is to large")
	}

	for i, b := range srcImg.Pix {
		if (i+1)%4 == 0 {
			destImg.Pix[i] = b
			continue
		}

		if bit, err := r.next(); err != nil {
			destImg.Pix[i] = b
		} else {
			if b%2 != 0 {
				b--
			}
			destImg.Pix[i] = b + bit
		}
	}

	fpo, err := os.Create(fout)
	if err != nil {
		fatal(err)
	}
	defer fpo.Close()

	if bmp.Encode(fpo, destImg) != nil {
		fatal(err)
	}
}

type bitReader struct {
	ptr  int
	data []byte
}

func newBitReader(file string) bitReader {
	msg, err := ioutil.ReadFile(file)
	if err != nil {
		fatal(err)
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(len(msg)))
	binary.Write(&buf, binary.BigEndian, adler32.Checksum(msg))
	buf.Write(msg)

	return bitReader{0, buf.Bytes()}
}

func (br *bitReader) next() (byte, error) {
	i := br.ptr / 8
	if i >= len(br.data) {
		return 0, io.EOF
	}

	bit := uint8(br.ptr % 8)
	b := br.data[i]
	br.ptr++

	return (b & (0x80 >> bit)) >> (7 - bit), nil
}
