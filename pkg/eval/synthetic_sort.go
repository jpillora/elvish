package eval

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"
	"strings"
)

// ReadString avoids Scanner's token-size limit and retains CR bytes in CRLF
// input. Like Unix sort and uniq, callers output a newline for the last record
// even when the input did not end with one.
func syntheticReadLines(in io.Reader, process func(string) error) error {
	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			if err := process(strings.TrimSuffix(line, "\n")); err != nil {
				return err
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

func syntheticTextOutput(out io.Writer, path string, process func(io.Writer) error) error {
	if path == "" {
		return process(out)
	}
	f, err := os.Create(path)
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

func syntheticFoldASCII(text string) string {
	folded := []byte(text)
	for i, b := range folded {
		if b >= 'a' && b <= 'z' {
			folded[i] = b - 'a' + 'A'
		}
	}
	return string(folded)
}

type syntheticSortOptions struct {
	numeric, reverse, unique, fold, blanks, stable bool
	output                                         string
}

func syntheticSortArgs(args []string) (syntheticSortOptions, []string, error) {
	var options syntheticSortOptions
	var paths []string
	longFlags := map[string]string{
		"--numeric-sort": "n", "--reverse": "r", "--unique": "u",
		"--ignore-case": "f", "--ignore-leading-blanks": "b", "--stable": "s",
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
		if flag, ok := longFlags[arg]; ok {
			arg = "-" + flag
		} else if strings.HasPrefix(arg, "--output=") {
			options.output = strings.TrimPrefix(arg, "--output=")
			if options.output == "" {
				return options, nil, errors.New("sort: output path required")
			}
			continue
		} else if arg == "--output" {
			arg = "-o"
		} else if strings.HasPrefix(arg, "--") {
			return options, nil, fmt.Errorf("sort: unsupported option %s", arg)
		}
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'n':
				options.numeric = true
			case 'r':
				options.reverse = true
			case 'u':
				options.unique = true
			case 'f':
				options.fold = true
			case 'b':
				options.blanks = true
			case 's':
				options.stable = true
			case 'o':
				value := arg[j+1:]
				if value == "" {
					i++
					if i == len(args) {
						return options, nil, errors.New("sort: -o requires an output path")
					}
					value = args[i]
				}
				if value == "" {
					return options, nil, errors.New("sort: output path required")
				}
				options.output = value
				j = len(arg)
			default:
				return options, nil, fmt.Errorf("sort: unsupported option -%c", arg[j])
			}
		}
	}
	return options, paths, nil
}

// Decimal prefix comparison uses exact rationals, so integers larger than a
// float64's precision and long decimal fractions retain their Unix ordering.
// In numeric mode, non-numbers compare as zero; '+' and exponents are not part
// of the numeric prefix.
func syntheticSortNumber(line string) *big.Rat {
	line = strings.TrimLeft(line, " \t")
	end := 0
	if strings.HasPrefix(line, "-") {
		end++
	}
	digits := 0
	for end < len(line) && line[end] >= '0' && line[end] <= '9' {
		end++
		digits++
	}
	if end < len(line) && line[end] == '.' {
		end++
		for end < len(line) && line[end] >= '0' && line[end] <= '9' {
			end++
			digits++
		}
	}
	if digits > 0 {
		if number, ok := new(big.Rat).SetString(line[:end]); ok {
			return number
		}
	}
	return new(big.Rat)
}

type syntheticSortLine struct {
	text, key string
	number    *big.Rat
}

func syntheticSort(in io.Reader, out io.Writer, args []string) error {
	options, paths, err := syntheticSortArgs(args)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	var lines []syntheticSortLine
	for _, path := range paths {
		if err := syntheticTextInput(in, path, func(reader io.Reader) error {
			return syntheticReadLines(reader, func(text string) error {
				line := syntheticSortLine{text: text, key: text}
				if options.blanks {
					line.key = strings.TrimLeft(line.key, " \t")
				}
				if options.fold {
					line.key = syntheticFoldASCII(line.key)
				}
				if options.numeric {
					line.number = syntheticSortNumber(text)
				}
				lines = append(lines, line)
				return nil
			})
		}); err != nil {
			return err
		}
	}
	compare := func(a, b syntheticSortLine) int {
		if options.numeric {
			return a.number.Cmp(b.number)
		}
		return strings.Compare(a.key, b.key)
	}
	sort.SliceStable(lines, func(i, j int) bool {
		cmp := compare(lines[i], lines[j])
		if cmp == 0 && !options.stable && !options.unique {
			cmp = strings.Compare(lines[i].text, lines[j].text)
		}
		if options.reverse {
			return cmp > 0
		}
		return cmp < 0
	})
	// Open the output only after every input has been read successfully. This
	// supports sort -o FILE FILE without truncating the source first.
	return syntheticTextOutput(out, options.output, func(writer io.Writer) error {
		for i, line := range lines {
			if options.unique && i > 0 && compare(line, lines[i-1]) == 0 {
				continue
			}
			if _, err := fmt.Fprintln(writer, line.text); err != nil {
				return err
			}
		}
		return nil
	})
}

func syntheticUniq(in io.Reader, out io.Writer, args []string) error {
	// Expand long options before the shared short-option parser, respecting --.
	aliases := map[string]string{"--count": "-c", "--repeated": "-d", "--unique": "-u", "--ignore-case": "-i"}
	normalized := append([]string(nil), args...)
	for i, arg := range normalized {
		if arg == "--" {
			break
		}
		if short, ok := aliases[arg]; ok {
			normalized[i] = short
		}
	}
	flags, paths, err := syntheticFlags("uniq", normalized, "cdui")
	if err != nil {
		return err
	}
	if len(paths) > 2 {
		return errors.New("uniq: accepts at most one input and one output path")
	}
	input, output := "-", ""
	if len(paths) > 0 {
		input = paths[0]
	}
	if len(paths) > 1 {
		output = paths[1]
	}
	if output == "-" {
		output = ""
	}
	if input != "-" && output != "" && output != "-" {
		inputInfo, inErr := os.Stat(input)
		outputInfo, outErr := os.Stat(output)
		if inErr == nil && outErr == nil && os.SameFile(inputInfo, outputInfo) {
			return errors.New("uniq: input and output are the same file")
		}
	}
	return syntheticTextInput(in, input, func(reader io.Reader) error {
		return syntheticTextOutput(out, output, func(writer io.Writer) error {
			var current, key string
			var count int64
			flush := func() error {
				if count == 0 || flags['d'] && count == 1 || flags['u'] && count > 1 {
					return nil
				}
				if flags['c'] {
					_, err := fmt.Fprintf(writer, "%7d %s\n", count, current)
					return err
				}
				_, err := fmt.Fprintln(writer, current)
				return err
			}
			if err := syntheticReadLines(reader, func(line string) error {
				lineKey := line
				if flags['i'] {
					lineKey = syntheticFoldASCII(lineKey)
				}
				if count > 0 && key == lineKey {
					count++
					return nil
				}
				if err := flush(); err != nil {
					return err
				}
				current, key, count = line, lineKey, 1
				return nil
			}); err != nil {
				return err
			}
			return flush()
		})
	})
}
