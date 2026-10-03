package eval

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

type syntheticSliceOptions struct {
	count       int64
	bytes       bool
	fromStart   bool
	excludeLast bool
	headers     int // -1: never; 0: multiple inputs; 1: always
}

func syntheticSliceArgs(command string, args []string) (syntheticSliceOptions, []string, error) {
	options := syntheticSliceOptions{count: 10}
	var paths []string
	parseCount := func(value string, bytes bool) error {
		digits := value
		if strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
			digits = value[1:]
		}
		count, err := strconv.ParseUint(digits, 10, 63)
		if err != nil {
			return fmt.Errorf("%s: invalid count %q", command, value)
		}
		options.count, options.bytes = int64(count), bytes
		options.fromStart = command == "tail" && strings.HasPrefix(value, "+")
		options.excludeLast = command == "head" && strings.HasPrefix(value, "-")
		return nil
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			paths = append(paths, arg)
			continue
		}
		if len(arg) > 1 && arg[1] >= '0' && arg[1] <= '9' {
			if err := parseCount(arg[1:], false); err != nil {
				return options, nil, err
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			option, value, hasValue := strings.Cut(arg, "=")
			switch option {
			case "--quiet", "--silent", "--verbose":
				if hasValue {
					return options, nil, fmt.Errorf("%s: %s does not accept a value", command, option)
				}
				options.headers = -1
				if option == "--verbose" {
					options.headers = 1
				}
			case "--lines", "--bytes":
				if !hasValue {
					i++
					if i == len(args) {
						return options, nil, fmt.Errorf("%s: %s requires a count", command, option)
					}
					value = args[i]
				}
				if err := parseCount(value, option == "--bytes"); err != nil {
					return options, nil, err
				}
			default:
				return options, nil, fmt.Errorf("%s: unsupported option %s", command, option)
			}
			continue
		}
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'q':
				options.headers = -1
			case 'v':
				options.headers = 1
			case 'n', 'c':
				value := arg[j+1:]
				if value == "" {
					i++
					if i == len(args) {
						return options, nil, fmt.Errorf("%s: -%c requires a count", command, arg[j])
					}
					value = args[i]
				}
				if err := parseCount(value, arg[j] == 'c'); err != nil {
					return options, nil, err
				}
				j = len(arg)
			default:
				return options, nil, fmt.Errorf("%s: unsupported option -%c", command, arg[j])
			}
		}
	}
	return options, paths, nil
}

// syntheticTextInput only owns readers opened for named files. In particular,
// it never closes the frame's stdin, which may appear more than once as "-".
func syntheticTextInput(in io.Reader, path string, process func(io.Reader) error) error {
	if path == "-" {
		return process(in)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = process(f)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func syntheticSliceText(command string, in io.Reader, out io.Writer, args []string) error {
	options, paths, err := syntheticSliceArgs(command, args)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	headers := options.headers > 0 || options.headers == 0 && len(paths) > 1
	for i, path := range paths {
		if err := syntheticTextInput(in, path, func(reader io.Reader) error {
			if headers {
				if i > 0 {
					if _, err := fmt.Fprintln(out); err != nil {
						return err
					}
				}
				name := path
				if name == "-" {
					name = "standard input"
				}
				if _, err := fmt.Fprintf(out, "==> %s <==\n", name); err != nil {
					return err
				}
			}
			if options.bytes {
				return syntheticSliceBytes(command, reader, out, options)
			}
			return syntheticSliceLines(command, reader, out, options)
		}); err != nil {
			return err
		}
	}
	return nil
}

func syntheticSliceLines(command string, in io.Reader, out io.Writer, options syntheticSliceOptions) error {
	if options.count == 0 {
		if options.excludeLast || options.fromStart {
			_, err := io.Copy(out, in)
			return err
		}
		return nil
	}
	reader := bufio.NewReader(in)
	var ring []string
	next := 0
	for lineNo := int64(1); ; lineNo++ {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			switch {
			case options.fromStart:
				if lineNo >= options.count {
					if _, err := io.WriteString(out, line); err != nil {
						return err
					}
					if err != nil {
						if errors.Is(err, io.EOF) {
							return nil
						}
						return err
					}
					_, err := io.Copy(out, reader)
					return err
				}
			case command == "head" && !options.excludeLast:
				if _, err := io.WriteString(out, line); err != nil {
					return err
				}
				if lineNo == options.count {
					if err != nil && !errors.Is(err, io.EOF) {
						return err
					}
					return nil
				}
			default:
				if int64(len(ring)) < options.count {
					ring = append(ring, line)
				} else {
					if options.excludeLast {
						if _, err := io.WriteString(out, ring[next]); err != nil {
							return err
						}
					}
					ring[next] = line
					next = (next + 1) % len(ring)
				}
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return err
			}
			break
		}
	}
	if command == "tail" {
		for i := 0; i < len(ring); i++ {
			if _, err := io.WriteString(out, ring[(next+i)%len(ring)]); err != nil {
				return err
			}
		}
	}
	return nil
}

func syntheticSliceBytes(command string, in io.Reader, out io.Writer, options syntheticSliceOptions) error {
	if options.count == 0 {
		if options.excludeLast || options.fromStart {
			_, err := io.Copy(out, in)
			return err
		}
		return nil
	}
	if options.fromStart {
		skip := options.count - 1
		if skip > 0 {
			if _, err := io.CopyN(io.Discard, in, skip); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
		_, err := io.Copy(out, in)
		return err
	}
	if command == "head" && !options.excludeLast {
		_, err := io.CopyN(out, in, options.count)
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	var chunks [][]byte
	var kept int64
	buf := make([]byte, 32768)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			chunks = append(chunks, append([]byte(nil), buf[:n]...))
			kept += int64(n)
			for kept > options.count {
				drop := min(kept-options.count, int64(len(chunks[0])))
				if options.excludeLast {
					if _, err := out.Write(chunks[0][:int(drop)]); err != nil {
						return err
					}
				}
				chunks[0] = chunks[0][int(drop):]
				if len(chunks[0]) == 0 {
					chunks[0] = nil
					chunks = chunks[1:]
				}
				kept -= drop
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return err
			}
			break
		}
	}
	if command == "tail" {
		for _, chunk := range chunks {
			if _, err := out.Write(chunk); err != nil {
				return err
			}
		}
	}
	return nil
}

type syntheticWordCounts struct {
	lines, words, chars, bytes int64
}

func syntheticCountWords(in io.Reader) (syntheticWordCounts, error) {
	var counts syntheticWordCounts
	reader := bufio.NewReader(in)
	inWord := false
	for {
		r, size, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return counts, nil
			}
			return counts, err
		}
		counts.bytes += int64(size)
		counts.chars++
		if r == '\n' {
			counts.lines++
		}
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			counts.words++
			inWord = true
		}
	}
}

func syntheticWc(in io.Reader, out io.Writer, args []string) error {
	flags, paths, err := syntheticFlags("wc", args, "lwcm")
	if err != nil {
		return err
	}
	if len(flags) == 0 {
		flags['l'], flags['w'], flags['c'] = true, true, true
	}
	named := len(paths) > 0
	if !named {
		paths = []string{"-"}
	}
	printCounts := func(counts syntheticWordCounts, name string) error {
		var fields []string
		for _, field := range []struct {
			flag  rune
			value int64
		}{{'l', counts.lines}, {'w', counts.words}, {'m', counts.chars}, {'c', counts.bytes}} {
			if flags[field.flag] {
				fields = append(fields, strconv.FormatInt(field.value, 10))
			}
		}
		if name != "" {
			fields = append(fields, name)
		}
		_, err := fmt.Fprintln(out, strings.Join(fields, " "))
		return err
	}
	var total syntheticWordCounts
	for _, path := range paths {
		var counts syntheticWordCounts
		if err := syntheticTextInput(in, path, func(reader io.Reader) error {
			var err error
			counts, err = syntheticCountWords(reader)
			return err
		}); err != nil {
			return err
		}
		total.lines += counts.lines
		total.words += counts.words
		total.chars += counts.chars
		total.bytes += counts.bytes
		name := ""
		if named {
			name = path
		}
		if err := printCounts(counts, name); err != nil {
			return err
		}
	}
	if len(paths) > 1 {
		return printCounts(total, "total")
	}
	return nil
}
