package gexpect

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func mockExpectFromString(buffer string) *ExpectIO {
	b := bytes.NewBufferString(buffer)
	return NewExpectIO(b, b)
}

func TestEmptySearchString(t *testing.T) {
	t.Logf("Testing empty search string...")
	exp := mockExpectFromString("Hello World")
	err := exp.Expect("")
	if err != ErrEmptySearch {
		t.Fatalf("Expected empty search error, got %v", err)
	}
}

func TestHelloWorld(t *testing.T) {
	t.Logf("Testing Hello World... ")
	exp := mockExpectFromString("Hello World")
	err := exp.Expect("Hello World")
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoubleHelloWorld(t *testing.T) {
	t.Logf("Testing Double Hello World... ")
	exp := mockExpectFromString("Hello World\nHello\nHi")
	err := exp.Expect("Hello World")
	if err != nil {
		t.Fatal(err)
	}
	err = exp.Expect("Hello")
	if err != nil {
		t.Fatal(err)
	}
	err = exp.Expect("Hi")
	if err != nil {
		t.Fatal(err)
	}
}

func TestHelloWorldFailureCase(t *testing.T) {
	t.Logf("Testing Hello World Failure case... ")
	exp := mockExpectFromString("Hello World")
	err := exp.Expect("YOU WILL NEVER FIND ME")
	if err != nil {
		return
	}
	t.Fatal("Expected an error for TestHelloWorldFailureCase")
}

func TestBiChannel(t *testing.T) {

	t.Logf("Testing BiChannel screen... ")

	pipeReader, pipeWriter := io.Pipe()

	exp := NewExpectIO(pipeReader, pipeWriter)

	sender, receiver := exp.AsyncInteractChannels()
	wait := func(str string) {
		for {
			msg, open := <-receiver
			if !open {
				return
			}
			if strings.Contains(msg, str) {
				return
			}
		}
	}

	endlChar := fmt.Sprintln("")
	sender <- fmt.Sprintf("echo%v", endlChar)
	wait("echo")
	sender <- fmt.Sprintf("echo2%v", endlChar)
	wait("echo2")
}

var regexMatchTests = []struct {
	re   string
	good string
	bad  string
}{
	{`a`, `a`, `b`},
	{`.b`, `ab`, `ac`},
	{`a+hello`, `aaaahello`, `bhello`},
	{`(hello|world)`, `hello`, `unknown`},
	{`(hello|world)`, `world`, `unknown`},
	{"\u00a9", "\u00a9", `unknown`}, // 2 bytes long unicode character "copyright sign"
}

func TestRegexMatch(t *testing.T) {
	t.Logf("Testing Regular Expression Matching... ")
	for _, tt := range regexMatchTests {
		runTest := func(input string) bool {
			var match bool
			exp := mockExpectFromString(input)
			match, err := exp.ExpectRegex(tt.re)
			if err != nil {
				t.Fatal(err)
			}
			return match
		}
		if !runTest(tt.good) {
			t.Errorf("Regex Not matching [%#q] with pattern [%#q]", tt.good, tt.re)
		}
		if runTest(tt.bad) {
			t.Errorf("Regex Matching [%#q] with pattern [%#q]", tt.bad, tt.re)
		}
	}
}

var regexFindTests = []struct {
	re      string
	input   string
	matches []string
}{
	{`he(l)lo wo(r)ld`, `hello world`, []string{"hello world", "l", "r"}},
	{`(a)`, `a`, []string{"a", "a"}},
	{`so.. (hello|world)`, `so.. hello`, []string{"so.. hello", "hello"}},
	{`(a+)hello`, `aaaahello`, []string{"aaaahello", "aaaa"}},
	{`\d+ (\d+) (\d+)`, `123 456 789`, []string{"123 456 789", "456", "789"}},
	{`\d+ (\d+) (\d+)`, "\u00a9 123 456 789 \u00a9", []string{"123 456 789", "456", "789"}}, // check unicode characters
}

func TestRegexFind(t *testing.T) {
	t.Logf("Testing Regular Expression Search... ")
	for _, tt := range regexFindTests {
		runTest := func(input string) []string {
			exp := mockExpectFromString(input)
			matches, err := exp.ExpectRegexFind(tt.re)
			if err != nil {
				t.Fatal(err)
			}
			return matches
		}
		matches := runTest(tt.input)
		if len(matches) != len(tt.matches) {
			t.Fatalf("Regex not producing the expected number of patterns.. got[%d] ([%s]) expected[%d] ([%s])",
				len(matches), strings.Join(matches, ","),
				len(tt.matches), strings.Join(tt.matches, ","))
		}
		for i, _ := range matches {
			if matches[i] != tt.matches[i] {
				t.Errorf("Regex Expected group [%s] and got group [%s] with pattern [%#q] and input [%s]",
					tt.matches[i], matches[i], tt.re, tt.input)
			}
		}
	}
}

func TestReadLine(t *testing.T) {
	t.Logf("Testing ReadLine...")

	exp := mockExpectFromString("foo\r\nbar\r\n")

	s, err := exp.ReadLine()

	if err != nil {
		t.Fatal(err)
	}
	if s != "foo\r" {
		t.Fatalf("expected 'foo\\r', got '%s'", s)
	}
	s, err = exp.ReadLine()
	if err != nil {
		t.Fatal(err)
	}
	if s != "bar\r" {
		t.Fatalf("expected 'bar\\r', got '%s'", s)
	}
}

func TestRegexWithOutput(t *testing.T) {
	t.Logf("Testing Regular Expression search with output...")

	s := "You will not find me"
	exp := mockExpectFromString(s)

	searchPattern := `I should not find you`
	result, out, err := exp.ExpectRegexFindWithOutput(searchPattern)
	if err == nil {
		t.Fatalf("Shouldn't have found `%v` in `%v`", searchPattern, out)
	}
	if s != out {
		t.Fatalf("Child output didn't match: %s", out)
	}

	exp = mockExpectFromString("You will find me\r\n")

	searchPattern = `.*(You will).*`
	result, out, err = exp.ExpectRegexFindWithOutput(searchPattern)
	if err != nil || result[1] != "You will" {
		t.Fatalf("Did not find pattern `%v` in `%v'\n", searchPattern, out)
	}
}

func TestRegexTimeoutWithOutput(t *testing.T) {
	t.Logf("Testing Regular Expression search with timeout and output...")

	seconds := 2
	timeout := time.Duration(seconds-1) * time.Second

	pipeReader, pipeWriter := io.Pipe()

	exp := NewExpectIO(pipeReader, nil)

	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		pipeWriter.Write([]byte("You find me\n"))
	}()

	searchPattern := `find me`
	result, out, err := exp.ExpectTimeoutRegexFindWithOutput(searchPattern, timeout)
	if err == nil {
		t.Fatalf("Shouldn't have finished call with result: %v", result)
	}

	seconds = 2
	timeout = time.Duration(seconds+1) * time.Second

	pipeReader, pipeWriter = io.Pipe()
	exp = NewExpectIO(pipeReader, nil)

	go func() {
		time.Sleep(time.Duration(seconds) * time.Second)
		pipeWriter.Write([]byte("You find me\n"))
	}()

	searchPattern = `find me`
	result, out, err = exp.ExpectTimeoutRegexFindWithOutput(searchPattern, timeout)
	if err != nil {
		t.Fatalf("Didn't find %v in output: %v", searchPattern, out)
	}
}

func TestRegexFindNoExcessBytes(t *testing.T) {
	t.Logf("Testing Regular Expressions returning output with no excess strings")
	repeats := 50
	tests := []struct {
		desc           string
		loopBody       string
		searchPattern  string
		expectFullTmpl string
		unmatchedData  string
	}{
		{
			desc:           `matching lines line by line with $ at the end of the regexp`,
			loopBody:       "prefix: %d line\n",
			searchPattern:  `(?m)^prefix:\s+(\d+) line\s??$`,
			expectFullTmpl: `prefix: %d line`,
			unmatchedData:  "\n",
			// the "$" char at the end of regexp does not
			// match the \n, so it is left as an unmatched
			// data
		},
		{
			desc:           `matching lines line by line with \n at the end of the regexp`,
			loopBody:       "prefix: %d line\r\n",
			searchPattern:  `(?m)^prefix:\s+(\d+) line\s??\n`,
			expectFullTmpl: `prefix: %d line`,
			unmatchedData:  "",
		},
		{
			desc:           `matching chunks in single line chunk by chunk`,
			loopBody:       "a %d b",
			searchPattern:  `a\s+(\d+)\s+b`,
			expectFullTmpl: `a %d b`,
			unmatchedData:  "",
		},
	}

	for _, tt := range tests {
		t.Logf("Test: %s", tt.desc)

		var buf bytes.Buffer
		for i := 1; i <= repeats; i++ {
			buf.WriteString(fmt.Sprintf(tt.loopBody, i))
		}

		exp := NewExpectIO(&buf, &buf)

		for i := 1; i <= repeats; i++ {
			matches, output, err := exp.ExpectRegexFindWithOutput(tt.searchPattern)
			if err != nil {
				t.Fatalf("Failed to get the match number %d: %v", i, err)
			}
			if len(matches) != 2 {
				t.Fatalf("Expected only 2 matches, got %d", len(matches))
			}
			full := strings.TrimSpace(matches[0])
			expFull := fmt.Sprintf(tt.expectFullTmpl, i)
			partial := matches[1]
			expPartial := fmt.Sprintf("%d", i)
			if full != expFull {
				t.Fatalf("Did not the expected full match %q, got %q", expFull, full)
			}
			if partial != expPartial {
				t.Fatalf("Did not the expected partial match %q, got %q", expPartial, partial)
			}
			// The output variable usually contains the
			// unmatched data followed by the whole match.
			// The first line is special as it has no data
			// preceding it.
			var expectedOutput string
			if i == 1 || tt.unmatchedData == "" {
				expectedOutput = matches[0]
			} else {
				expectedOutput = fmt.Sprintf("%s%s", tt.unmatchedData, matches[0])
			}
			if output != expectedOutput {
				t.Fatalf("The collected output %q should be the same as the whole match %q", output, expectedOutput)
			}
		}
	}
}

func TestBufferReadRune(t *testing.T) {
	tests := []struct {
		bufferContent []byte
		fileContent   []byte
		expectedRune  rune
	}{
		// unicode "copyright char" is \u00a9 is encoded as two bytes in utf8 0xc2 0xa9
		{[]byte{0xc2, 0xa9}, []byte{}, '\u00a9'}, // whole rune is already in buffer.b
		{[]byte{0xc2}, []byte{0xa9}, '\u00a9'},   // half of is in the buffer.b and another half still in buffer.f (file)
		{[]byte{}, []byte{0xc2, 0xa9}, '\u00a9'}, // whole rune is the file
		// some random noise in the end of file
		{[]byte{0xc2, 0xa9}, []byte{0x20, 0x20, 0x20, 0x20}, '\u00a9'},
		{[]byte{0xc2}, []byte{0xa9, 0x20, 0x20, 0x20, 0x20}, '\u00a9'},
		{[]byte{}, []byte{0xc2, 0xa9, 0x20, 0x20, 0x20, 0x20}, '\u00a9'},
	}

	for i, tt := range tests {

		// prepare tmp file with fileContent
		f, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		n, err := f.Write(tt.fileContent)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(tt.fileContent) {
			t.Fatal("expected fileContent written to temp file")
		}
		_, err = f.Seek(0, 0)
		if err != nil {
			t.Fatal(err)
		}

		// new buffer
		buf := buffer{rw: bufio.NewReadWriter(bufio.NewReader(f), bufio.NewWriter(f)), b: *bytes.NewBuffer(tt.bufferContent)}

		// call ReadRune
		r, size, err := buf.ReadRune()

		if r != tt.expectedRune {
			t.Fatalf("#%d: expected rune %+q but go is %+q", i, tt.expectedRune, r)
		}

		if size != len(string(tt.expectedRune)) {
			t.Fatalf("#%d: expected rune %d bytes long but got just %d bytes long", i, len(string(tt.expectedRune)), size)
		}

	}

}
