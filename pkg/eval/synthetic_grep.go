package eval

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type syntheticGrepOptions struct {
	mode                                            byte // G: basic; E: extended; F: fixed
	invert, fold, number, count, quiet, whole, only bool
	filenames, list                                 int
	sources                                         []syntheticGrepSource
	patternSources                                  bool
}

type syntheticGrepSource struct {
	value string
	file  bool
}

func syntheticGrepArgs(args []string) (syntheticGrepOptions, []string, error) {
	var options syntheticGrepOptions
	var paths []string
	aliases := map[string]string{
		"--basic-regexp": "G", "--extended-regexp": "E", "--fixed-strings": "F",
		"--invert-match": "v", "--ignore-case": "i", "--line-number": "n",
		"--count": "c", "--files-with-matches": "l", "--files-without-match": "L",
		"--quiet": "q", "--silent": "q", "--with-filename": "H", "--no-filename": "h",
		"--line-regexp": "x", "--only-matching": "o", "--text": "a",
	}
	addPatterns := func(value string) {
		options.patternSources = true
		for _, pattern := range strings.Split(value, "\n") {
			options.sources = append(options.sources, syntheticGrepSource{value: pattern})
		}
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
		if flag, ok := aliases[arg]; ok {
			arg = "-" + flag
		} else if strings.HasPrefix(arg, "--regexp=") {
			addPatterns(strings.TrimPrefix(arg, "--regexp="))
			continue
		} else if strings.HasPrefix(arg, "--file=") {
			arg = "-f" + strings.TrimPrefix(arg, "--file=")
			if arg == "-f" {
				return options, nil, errors.New("grep: pattern file path required")
			}
		} else if arg == "--regexp" {
			arg = "-e"
		} else if arg == "--file" {
			arg = "-f"
		} else if strings.HasPrefix(arg, "--") {
			return options, nil, fmt.Errorf("grep: unsupported option %s", arg)
		}
		for j := 1; j < len(arg); j++ {
			switch arg[j] {
			case 'G', 'E', 'F':
				if options.mode != 0 && options.mode != arg[j] {
					return options, nil, errors.New("grep: conflicting pattern modes")
				}
				options.mode = arg[j]
			case 'v':
				options.invert = true
			case 'i':
				options.fold = true
			case 'n':
				options.number = true
			case 'c':
				options.count = true
			case 'q':
				options.quiet = true
			case 'x':
				options.whole = true
			case 'o':
				options.only = true
			case 'H':
				options.filenames = 1
			case 'h':
				options.filenames = -1
			case 'l':
				options.list = 1
			case 'L':
				options.list = -1
			case 'a': // All inputs are processed as text.
			case 'e', 'f':
				value := arg[j+1:]
				if value == "" {
					i++
					if i == len(args) {
						return options, nil, fmt.Errorf("grep: -%c requires a value", arg[j])
					}
					value = args[i]
				}
				if arg[j] == 'e' {
					addPatterns(value)
				} else {
					options.patternSources = true
					// Pattern files are read by syntheticGrep, so -f - can consume
					// stdin before it becomes an input to the search itself.
					options.sources = append(options.sources, syntheticGrepSource{value: value, file: true})
				}
				j = len(arg)
			default:
				return options, nil, fmt.Errorf("grep: unsupported option -%c", arg[j])
			}
		}
	}
	if !options.patternSources {
		if len(paths) == 0 {
			return options, nil, errors.New("grep: pattern required")
		}
		addPatterns(paths[0])
		paths = paths[1:]
	}
	return options, paths, nil
}

// Basic regexp operators use backslashes where extended regexps do not.
// Unsupported BRE backreferences fail explicitly instead of matching a
// different pattern under Go's RE2 engine.
func syntheticGrepBasic(pattern string) (string, error) {
	var translated strings.Builder
	class, first := false, false
	for i := 0; i < len(pattern); i++ {
		b := pattern[i]
		if class && b == '\\' {
			translated.WriteString(`\\`)
			first = false
			continue
		}
		if b == '\\' {
			i++
			if i == len(pattern) {
				return "", errors.New("grep: trailing backslash in pattern")
			}
			next := pattern[i]
			switch {
			case strings.ContainsRune("+?(){}|", rune(next)):
				translated.WriteByte(next)
			case next >= '1' && next <= '9':
				return "", errors.New("grep: regexp backreferences are not supported")
			case next == '<' || next == '>':
				return "", errors.New("grep: directional word boundaries are not supported")
			case strings.ContainsRune("bBwWsS", rune(next)):
				translated.WriteByte('\\')
				translated.WriteByte(next)
			default:
				translated.WriteString(regexp.QuoteMeta(string(next)))
			}
			continue
		}
		if class {
			if b == '[' && i+1 < len(pattern) && strings.ContainsRune(":.=", rune(pattern[i+1])) {
				end := strings.Index(pattern[i+2:], string(pattern[i+1])+"]")
				if end >= 0 {
					if pattern[i+1] != ':' {
						return "", errors.New("grep: collating symbols and equivalence classes are not supported")
					}
					end += i + 4
					translated.WriteString(pattern[i:end])
					i = end - 1
					first = false
					continue
				}
			}
			if b == ']' && first {
				translated.WriteString(`\]`)
				first = false
			} else {
				translated.WriteByte(b)
				if b == ']' {
					class = false
				}
				if b != '^' || !first {
					first = false
				}
			}
			continue
		}
		if b == '[' {
			class, first = true, true
			translated.WriteByte(b)
		} else if b == '^' && i != 0 && !strings.HasSuffix(pattern[:i], `\(`) && !strings.HasSuffix(pattern[:i], `\|`) ||
			b == '$' && i != len(pattern)-1 && !strings.HasPrefix(pattern[i+1:], `\)`) && !strings.HasPrefix(pattern[i+1:], `\|`) {
			translated.WriteByte('\\')
			translated.WriteByte(b)
		} else if strings.ContainsRune("+?(){}|", rune(b)) {
			translated.WriteByte('\\')
			translated.WriteByte(b)
		} else {
			translated.WriteByte(b)
		}
	}
	return translated.String(), nil
}

func syntheticGrepRegexp(options syntheticGrepOptions, patterns []string) (*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil // An empty pattern file matches no lines.
	}
	var expressions []string
	for _, pattern := range patterns {
		var expression string
		var err error
		switch options.mode {
		case 'F':
			expression = regexp.QuoteMeta(pattern)
		case 'E':
			expression = pattern
		default:
			expression, err = syntheticGrepBasic(pattern)
			if err != nil {
				return nil, err
			}
		}
		expressions = append(expressions, "(?:"+expression+")")
	}
	expression := strings.Join(expressions, "|")
	if options.whole {
		expression = "^(?:" + expression + ")$"
	}
	if options.fold {
		expression = "(?i)" + expression
	}
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return nil, fmt.Errorf("grep: invalid pattern: %w", err)
	}
	compiled.Longest()
	return compiled, nil
}

var errSyntheticGrepStop = errors.New("grep: stop reading")

type syntheticGrepWriteError struct{ err error }

func (e syntheticGrepWriteError) Error() string { return e.err.Error() }

func syntheticGrep(in io.Reader, out, diagnostic io.Writer, args []string) error {
	fail := func(err error) error {
		if _, writeErr := fmt.Fprintln(diagnostic, err); writeErr != nil {
			return writeErr
		}
		return syntheticCommandExit("grep", 2)
	}
	options, paths, err := syntheticGrepArgs(args)
	if err != nil {
		return fail(err)
	}
	var patterns []string
	for _, source := range options.sources {
		if source.file {
			path := source.value
			if err := syntheticTextInput(in, path, func(reader io.Reader) error {
				return syntheticReadLines(reader, func(line string) error {
					patterns = append(patterns, line)
					return nil
				})
			}); err != nil {
				return fail(fmt.Errorf("grep: %w", err))
			}
		} else {
			patterns = append(patterns, source.value)
		}
	}
	matcher, err := syntheticGrepRegexp(options, patterns)
	if err != nil {
		return fail(err)
	}
	if len(paths) == 0 {
		paths = []string{"-"}
	}
	filenames := options.filenames > 0 || options.filenames == 0 && len(paths) > 1
	anyMatch, anyError := false, false
	for _, path := range paths {
		name := path
		if path == "-" {
			name = "(standard input)"
		}
		var lineNo, count int64
		opened, found := false, false
		writeLine := func(text string) error {
			prefix := ""
			if filenames {
				prefix = name + ":"
			}
			if options.number {
				prefix += fmt.Sprintf("%d:", lineNo)
			}
			_, err := fmt.Fprintln(out, prefix+text)
			if err != nil {
				return syntheticGrepWriteError{err}
			}
			return nil
		}
		err := syntheticTextInput(in, path, func(reader io.Reader) error {
			opened = true
			return syntheticReadLines(reader, func(line string) error {
				lineNo++
				var matches [][]int
				if matcher != nil {
					if options.only && !options.count && !options.quiet && options.list == 0 {
						matches = matcher.FindAllStringIndex(line, -1)
					} else if match := matcher.FindStringIndex(line); match != nil {
						matches = [][]int{match}
					}
				}
				selected := len(matches) > 0
				if options.invert {
					selected = !selected
				}
				if !selected {
					return nil
				}
				found, anyMatch = true, true
				count++
				if options.quiet || options.list != 0 {
					return errSyntheticGrepStop
				}
				if options.count {
					return nil
				}
				if options.only {
					if !options.invert {
						for _, match := range matches {
							if match[0] != match[1] {
								if err := writeLine(line[match[0]:match[1]]); err != nil {
									return err
								}
							}
						}
					}
					return nil
				}
				return writeLine(line)
			})
		})
		if errors.Is(err, errSyntheticGrepStop) {
			if options.quiet {
				return nil
			}
			err = nil
		} else if err != nil {
			var writeErr syntheticGrepWriteError
			if errors.As(err, &writeErr) {
				return writeErr.err
			}
			if _, writeErr := fmt.Fprintf(diagnostic, "grep: %s: %v\n", name, err); writeErr != nil {
				return writeErr
			}
			anyError = true
		}
		if !options.quiet && options.list != 0 && err == nil {
			if options.list == 1 && found || options.list == -1 && !found {
				if _, err := fmt.Fprintln(out, name); err != nil {
					return err
				}
			}
		} else if !options.quiet && options.list == 0 && options.count && opened {
			prefix := ""
			if filenames {
				prefix = name + ":"
			}
			if _, err := fmt.Fprintf(out, "%s%d\n", prefix, count); err != nil {
				return err
			}
		}
	}
	if anyError {
		return syntheticCommandExit("grep", 2)
	}
	if !anyMatch {
		return syntheticCommandExit("grep", 1)
	}
	return nil
}
