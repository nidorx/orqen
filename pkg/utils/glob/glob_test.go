package glob

import (
	"testing"
)

func TestGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		valid   []string
		invalid []string
	}{
		// ====================
		// * SIMPLES (1-10)
		// ====================
		{
			name:    "star matches any non-sequence",
			pattern: "*.github.com",
			valid:   []string{"api.github.com", "www.github.com", "a.github.com", "123.github.com", ".github.com"},
			invalid: []string{"api.gi.hub.com", "github.com"},
		},
		{
			name:    "star in middle",
			pattern: "api.*.com",
			valid:   []string{"api.github.com", "api.v1.com", "api.test.com"},
			invalid: []string{"api.github.io", "apiv1.com", "api./.com"},
		},
		{
			name:    "star at end",
			pattern: "test.*",
			valid:   []string{"test.file", "test.txt", "test."},
			invalid: []string{"test", "testing", "mytest.file"},
		},
		{
			name:    "multiple stars",
			pattern: "*.*",
			valid:   []string{"file.txt", "a.b", "test.log", ".txt", "file.txt.bak"},
			invalid: []string{"file"},
		},
		{
			name:    "star matches empty string",
			pattern: "a*b",
			valid:   []string{"ab", "aXb", "a123b"},
			invalid: []string{"a", "b", "a/b"},
		},
		{
			name:    "star does not match separator",
			pattern: "src/*.js",
			valid:   []string{"src/test.js", "src/app.js"},
			invalid: []string{"src/path/test.js", "src/a/b.js"},
		},
		{
			name:    "star only pattern",
			pattern: "*",
			valid:   []string{"file", "test", "a", ""},
			invalid: []string{"path/to"},
		},
		{
			name:    "consecutive stars",
			pattern: "***",
			valid:   []string{"abc", "xyz", "123", "", "a/b"},
			invalid: []string{},
		},
		{
			name:    "star with numbers",
			pattern: "file*.txt",
			valid:   []string{"file1.txt", "file123.txt", "file.txt", "fileABC.txt"},
			invalid: []string{"file1/2.txt", "myfile.txt", "file.txt.bak"},
		},
		{
			name:    "star prefix and suffix",
			pattern: "*test*",
			valid:   []string{"test", "mytest", "testing", "mytestfile"},
			invalid: []string{"my/test", "test/file"},
		},

		// ====================
		// ** DOUBLE STAR (11-20)
		// ====================
		{
			name:    "doublestar matches recursive",
			pattern: "src/**/*.js",
			valid:   []string{"src/path/to/file.js", "src/path/file.js", "src/file.js", "src/a/b/c/d.js", "src/.js"},
			invalid: []string{"src/file.css", "test/file.js"},
		},
		{
			name:    "doublestar at start",
			pattern: "**/test.js",
			valid:   []string{"test.js", "src/test.js", "a/b/c/test.js", "path/to/test.js"},
			invalid: []string{"test.txt", "src/test.txt"},
		},
		{
			name:    "doublestar at end",
			pattern: "src/**",
			valid:   []string{"src/file.js", "src/path/file.js", "src/a/b/c/d.js", "src/test"},
			invalid: []string{"src", "other/file.js"},
		},
		{
			name:    "doublestar only",
			pattern: "**",
			valid:   []string{"file.js", "path/to/file.js", "a/b/c/d", "anything", ""},
			invalid: []string{},
		},
		{
			name:    "doublestar middle",
			pattern: "src/**/test.js",
			valid:   []string{"src/test.js", "src/a/test.js", "src/a/b/c/test.js"},
			invalid: []string{"src/test.txt", "other/test.js"},
		},
		{
			name:    "doublestar with extension",
			pattern: "**/*.log",
			valid:   []string{"file.log", "logs/app.log", "a/b/c/debug.log"},
			invalid: []string{"file.txt", "logs/app.txt"},
		},
		{
			name:    "doublestar before extension",
			pattern: "**/test.*",
			valid:   []string{"test.js", "dir/test.txt", "a/b/c/test.log", "dir/mytest.txt"},
			invalid: []string{"testfile.js", "dir/mytest"},
		},
		{
			name:    "doublestar multiple levels",
			pattern: "a/**/b/**/c.js",
			valid:   []string{"a/b/c.js", "a/x/b/c.js", "a/x/y/b/z/c.js"},
			invalid: []string{"a/c.js", "a/b/d.js", "x/a/b/c.js"},
		},
		{
			name:    "doublestar with star",
			pattern: "**/*test*.js",
			valid:   []string{"test.js", "mytest.js", "path/testfile.js", "a/b/c_test_d.js"},
			invalid: []string{"test.txt", "file.js", "path/test.txt"},
		},
		{
			name:    "doublestar empty directory",
			pattern: "src/**/file.js",
			valid:   []string{"src/file.js", "src/dir/file.js", "src/a/b/c/file.js"},
			invalid: []string{"file.js", "src/file.txt"},
		},

		// ====================
		// ? SINGLE CHAR (21-30)
		// ====================
		{
			name:    "question matches single char",
			pattern: "file?.txt",
			valid:   []string{"file1.txt", "fileA.txt", "file_.txt"},
			invalid: []string{"file.txt", "file12.txt", "fileAB.txt"},
		},
		{
			name:    "question in middle",
			pattern: "a?b",
			valid:   []string{"a1b", "aXb", "a_b"},
			invalid: []string{"ab", "a12b", "a/b"},
		},
		{
			name:    "question at start",
			pattern: "?test",
			valid:   []string{"atest", "1test", "_test"},
			invalid: []string{"test", "mytest"},
		},
		{
			name:    "question at end",
			pattern: "test?",
			valid:   []string{"test1", "testX", "test_"},
			invalid: []string{"test", "testing"},
		},
		{
			name:    "multiple questions",
			pattern: "???.txt",
			valid:   []string{"abc.txt", "123.txt", "XYZ.txt"},
			invalid: []string{"ab.txt", "abcd.txt", ".txt"},
		},
		{
			name:    "question with star",
			pattern: "?*.txt",
			valid:   []string{"a.txt", "ab.txt", "abc.txt", "1file.txt"},
			invalid: []string{".txt", "file/txt"},
		},
		{
			name:    "question does not match separator",
			pattern: "src/?/file.js",
			valid:   []string{"src/a/file.js", "src/1/file.js"},
			invalid: []string{"src/ab/file.js", "src//file.js", "src/file.js"},
		},
		{
			name:    "question only",
			pattern: "?",
			valid:   []string{"a", "1", "X"},
			invalid: []string{"", "ab", "a/"},
		},
		{
			name:    "question pattern no match",
			pattern: "??",
			valid:   []string{"ab", "12", "XY"},
			invalid: []string{"a", "abc", "a/"},
		},
		{
			name:    "question multiple",
			pattern: "a?b?c",
			valid:   []string{"a1b2c", "aXbYc", "a_b_c"},
			invalid: []string{"abc", "a12bc", "a/b/c"},
		},

		// ====================
		// [CHARACTER CLASS] (31-42)
		// ====================
		{
			name:    "bracket matches char in set",
			pattern: "file[abc].txt",
			valid:   []string{"filea.txt", "fileb.txt", "filec.txt"},
			invalid: []string{"filed.txt", "file.txt", "fileab.txt"},
		},
		{
			name:    "bracket with range",
			pattern: "file[0-9].txt",
			valid:   []string{"file0.txt", "file5.txt", "file9.txt"},
			invalid: []string{"filea.txt", "file10.txt", "file.txt"},
		},
		{
			name:    "bracket negation",
			pattern: "file[!0-9].txt",
			valid:   []string{"filea.txt", "fileX.txt", "file_.txt"},
			invalid: []string{"file0.txt", "file5.txt", "file.txt"},
		},
		{
			name:    "bracket multiple ranges",
			pattern: "[a-zA-Z].txt",
			valid:   []string{"a.txt", "Z.txt", "m.txt"},
			invalid: []string{"0.txt", "_.txt", ".txt"},
		},
		{
			name:    "bracket at start",
			pattern: "[abc]file",
			valid:   []string{"afile", "bfile", "cfile"},
			invalid: []string{"dfile", "file", "abfile"},
		},
		{
			name:    "bracket at end",
			pattern: "test[xyz]",
			valid:   []string{"testx", "testy", "testz"},
			invalid: []string{"test", "testa", "testxy"},
		},
		{
			name:    "bracket only",
			pattern: "[0-9]",
			valid:   []string{"0", "5", "9"},
			invalid: []string{"a", "10", ""},
		},
		{
			name:    "bracket no closing treated as literal",
			pattern: "file[ab",
			valid:   []string{"file[ab"},
			invalid: []string{"filea", "fileb", "file"},
		},
		{
			name:    "bracket with single char",
			pattern: "file[a].txt",
			valid:   []string{"filea.txt"},
			invalid: []string{"fileb.txt", "file.txt"},
		},
		{
			name:    "bracket mixed chars and ranges",
			pattern: "[a-z0-9].txt",
			valid:   []string{"a.txt", "5.txt", "z.txt", "A.txt"},
			invalid: []string{"_.txt", ".txt"},
		},
		{
			name:    "bracket with question",
			pattern: "[ab]?",
			valid:   []string{"a1", "bX", "a_", "by", "ab"},
			invalid: []string{"c1", "a"},
		},
		{
			name:    "bracket with star",
			pattern: "[ab]*",
			valid:   []string{"a", "b", "abc", "afile", "b123"},
			invalid: []string{"c", "cab", "1ab"},
		},

		// ====================
		// {ALTERNATIVES} (43-55)
		// ====================
		{
			name:    "brace matches alternative",
			pattern: "*.{js,ts}",
			valid:   []string{"file.js", "file.ts", "app.js", "main.ts", "file.js.ts"},
			invalid: []string{"file.py", "file.go"},
		},
		{
			name:    "brace at start",
			pattern: "{src,test}/*.js",
			valid:   []string{"src/app.js", "test/main.js", "src/test.js"},
			invalid: []string{"lib/app.js", "src/app.ts"},
		},
		{
			name:    "brace at end",
			pattern: "file.{txt,log}",
			valid:   []string{"file.txt", "file.log"},
			invalid: []string{"file.js", "file", "file.txt.log"},
		},
		{
			name:    "brace multiple alternatives",
			pattern: "*.{js,ts,jsx,tsx}",
			valid:   []string{"file.js", "file.ts", "file.jsx", "file.tsx"},
			invalid: []string{"file.py", "file.css"},
		},
		{
			name:    "brace with star inside",
			pattern: "{*.js,*.ts}",
			valid:   []string{"file.js", "app.ts", "main.js"},
			invalid: []string{"file.py", "file.js.bak"},
		},
		{
			name:    "brace nested",
			pattern: "src/{lib,test}/*.js",
			valid:   []string{"src/lib/app.js", "src/test/main.js"},
			invalid: []string{"src/other/app.js", "lib/app.js"},
		},
		{
			name:    "brace with numbers",
			pattern: "file{1,2,3}.txt",
			valid:   []string{"file1.txt", "file2.txt", "file3.txt"},
			invalid: []string{"file4.txt", "file.txt", "file12.txt"},
		},
		{
			name:    "brace with doublestar",
			pattern: "**/{README,LICENSE}.*",
			valid:   []string{"README.md", "LICENSE.txt", "src/README.md", "pkg/LICENSE.md", "readme.md"},
			invalid: []string{"NOTICE.md"},
		},
		{
			name:    "brace no closing treated as literal",
			pattern: "file{js",
			valid:   []string{"file{js"},
			invalid: []string{"filejs", "file"},
		},
		{
			name:    "brace single alternative",
			pattern: "{file}.txt",
			valid:   []string{"file.txt"},
			invalid: []string{"file", "files.txt"},
		},
		{
			name:    "brace with question",
			pattern: "file?.{js,ts}",
			valid:   []string{"file1.js", "fileA.ts"},
			invalid: []string{"file.js", "file1.py"},
		},
		{
			name:    "brace empty alternative",
			pattern: "file{,bak}",
			valid:   []string{"file", "filebak"},
			invalid: []string{"filetxt", "other"},
		},
		{
			name:    "brace complex pattern",
			pattern: "{src,pkg}/**/*.{go,mod}",
			valid:   []string{"src/main.go", "pkg/utils/file.go", "src/a/b/c.go", "pkg/go.mod"},
			invalid: []string{"lib/main.go", "src/main.py", "pkg/utils/mod"},
		},

		// ====================
		// CASE INSENSITIVE (56-65)
		// ====================
		{
			name:    "case insensitive literal",
			pattern: "README.md",
			valid:   []string{"README.md", "readme.md", "Readme.md", "rEaDmE.Md"},
			invalid: []string{"README.txt", "readme"},
		},
		{
			name:    "case insensitive star",
			pattern: "*.JS",
			valid:   []string{"file.JS", "file.js", "file.Js", "app.JS"},
			invalid: []string{"file.ts", "file.py"},
		},
		{
			name:    "case insensitive doublestar",
			pattern: "**/TEST.*",
			valid:   []string{"TEST.js", "test.txt", "path/Test.log", "a/b/c/TEST.md", "mytest.js"},
			invalid: []string{"testing"},
		},
		{
			name:    "case insensitive question",
			pattern: "file?.TXT",
			valid:   []string{"file1.TXT", "fileA.txt", "file_.Txt"},
			invalid: []string{"file.TXT", "file12.TXT"},
		},
		{
			name:    "case insensitive bracket",
			pattern: "[abc].TXT",
			valid:   []string{"a.TXT", "B.txt", "c.Txt"},
			invalid: []string{"d.TXT", "ab.TXT"},
		},
		{
			name:    "case insensitive brace",
			pattern: "*.{JS,TS}",
			valid:   []string{"file.JS", "file.js", "file.TS", "app.Ts", "file.JS.ts"},
			invalid: []string{"file.py"},
		},
		{
			name:    "case insensitive mixed",
			pattern: "SRC/**/*TEST*.JS",
			valid:   []string{"SRC/TEST.JS", "src/mytest.js", "SRC/a/b/UNITTEST.Js"},
			invalid: []string{"SRC/test.txt", "other/TEST.JS"},
		},
		{
			name:    "case insensitive only",
			pattern: "test",
			valid:   []string{"test", "TEST", "Test", "tEsT"},
			invalid: []string{"testing", "mytest"},
		},
		{
			name:    "case insensitive extension",
			pattern: "*.LOG",
			valid:   []string{"app.LOG", "app.log", "app.Log", "debug.LOG"},
			invalid: []string{"app.txt", "app.LOG.bak"},
		},
		{
			name:    "case insensitive path",
			pattern: "SRC/FILE.TXT",
			valid:   []string{"SRC/FILE.TXT", "src/file.txt", "Src/File.Txt"},
			invalid: []string{"SRC/FILE.TXT.bak", "SRC/FILE"},
		},

		// ====================
		// WINDOWS PATH SEPARATORS (66-75)
		// ====================
		{
			name:    "backslash normalized to forward",
			pattern: "src\\file.js",
			valid:   []string{"src/file.js"},
			invalid: []string{"src/other/file.js", "file.js"},
		},
		{
			name:    "backslash with star",
			pattern: "src\\*.js",
			valid:   []string{"src/test.js", "src/app.js"},
			invalid: []string{"src/path/test.js", "src/file.txt"},
		},
		{
			name:    "backslash doublestar",
			pattern: "src\\**\\*.js",
			valid:   []string{"src/file.js", "src/path/file.js", "src/a/b/c.js"},
			invalid: []string{"src/file.txt", "other/file.js"},
		},
		{
			name:    "mixed separators",
			pattern: "src\\path/to\\file.js",
			valid:   []string{"src/path/to/file.js"},
			invalid: []string{"src/path/file.js"},
		},
		{
			name:    "backslash question",
			pattern: "src\\?\\file.js",
			valid:   []string{"src/a/file.js", "src/1/file.js"},
			invalid: []string{"src/ab/file.js", "src/file.js"},
		},
		{
			name:    "backslash brace",
			pattern: "src\\*.{js,ts}",
			valid:   []string{"src/file.js", "src/app.ts", "src/main.js"},
			invalid: []string{"src/file.py", "lib/file.js"},
		},
		{
			name:    "backslash bracket",
			pattern: "src\\[abc]\\file.js",
			valid:   []string{"src/a/file.js", "src/b/file.js"},
			invalid: []string{"src/d/file.js", "src/ab/file.js"},
		},
		{
			name:    "double backslash",
			pattern: "src\\\\file.js",
			valid:   []string{"src//file.js"},
			invalid: []string{"src/file.js"},
		},
		{
			name:    "backslash at start",
			pattern: "\\root\\file.js",
			valid:   []string{"/root/file.js"},
			invalid: []string{"root/file.js"},
		},
		{
			name:    "backslash recursive",
			pattern: "**\\test\\*.js",
			valid:   []string{"test/file.js", "src/test/app.js", "a/b/test/main.js"},
			invalid: []string{"test/file.txt", "src/main.js"},
		},

		// ====================
		// ESCAPING SPECIAL CHARS (76-85)
		// ====================
		{
			name:    "dot escaped",
			pattern: "file.min.js",
			valid:   []string{"file.min.js", "FILE.MIN.JS", "File.Min.Js"},
			invalid: []string{"fileXminXjs", "file-min-js"},
		},
		{
			name:    "plus escaped",
			pattern: "file+test.js",
			valid:   []string{"file+test.js", "FILE+TEST.JS"},
			invalid: []string{"filetest.js", "file test.js"},
		},
		{
			name:    "parentheses escaped",
			pattern: "test(1).js",
			valid:   []string{"test(1).js", "TEST(1).JS"},
			invalid: []string{"test1.js", "test.js"},
		},
		{
			name:    "caret escaped",
			pattern: "file^test.txt",
			valid:   []string{"file^test.txt", "FILE^TEST.TXT"},
			invalid: []string{"filetest.txt", "file test.txt"},
		},
		{
			name:    "dollar escaped",
			pattern: "price$.txt",
			valid:   []string{"price$.txt", "PRICE$.TXT"},
			invalid: []string{"price.txt", "price"},
		},
		{
			name:    "pipe escaped",
			pattern: "a|b.txt",
			valid:   []string{"a|b.txt", "A|B.TXT"},
			invalid: []string{"ab.txt", "a.txt", "b.txt"},
		},
		{
			name:    "multiple special chars",
			pattern: "file(1).min.js",
			valid:   []string{"file(1).min.js", "FILE(1).MIN.JS"},
			invalid: []string{"file1.min.js", "file().min.js"},
		},
		{
			name:    "all special chars",
			pattern: "a.b+c(d)^e$f|g.txt",
			valid:   []string{"a.b+c(d)^e$f|g.txt", "A.B+C(D)^E$F|G.TXT"},
			invalid: []string{"ab.txt", "a b+c(d)^e$f|g.txt"},
		},
		{
			name:    "dot star literal",
			pattern: "*.min.js",
			valid:   []string{"app.min.js", "bundle.min.js", "APP.MIN.JS"},
			invalid: []string{"app.min.txt", "appmin.js", "app.min"},
		},
		{
			name:    "complex escaping",
			pattern: "test(1).js",
			valid:   []string{"test(1).js", "Test(1).Js"},
			invalid: []string{"test1.js", "test.js", "test(2).js"},
		},

		// ====================
		// EDGE CASES (86-95)
		// ====================
		{
			name:    "empty pattern",
			pattern: "",
			valid:   []string{""},
			invalid: []string{"a", "file.txt"},
		},
		{
			name:    "exact match no wildcards",
			pattern: "src/main.go",
			valid:   []string{"src/main.go", "SRC/MAIN.GO", "Src/Main.Go"},
			invalid: []string{"src/main.go.bak", "src/other.go", "main.go"},
		},
		{
			name:    "star empty match",
			pattern: "a*b",
			valid:   []string{"ab", "a1b", "aXYb"},
			invalid: []string{"a/b", "a", "b"},
		},
		{
			name:    "doublestar empty match",
			pattern: "a/**/b",
			valid:   []string{"a/b", "a/x/b", "a/x/y/b", "a//b"},
			invalid: []string{"ab"},
		},
		{
			name:    "question no match empty",
			pattern: "a?b",
			valid:   []string{"a1b", "aXb"},
			invalid: []string{"ab", "a12b"},
		},
		{
			name:    "only separators",
			pattern: "/",
			valid:   []string{"/"},
			invalid: []string{"", "a", "a/"},
		},
		{
			name:    "multiple separators",
			pattern: "///",
			valid:   []string{"///"},
			invalid: []string{"/", "//", "a//"},
		},
		{
			name:    "unicode chars",
			pattern: "arquivo*.txt",
			valid:   []string{"arquivo.txt", "arquivo1.txt", "ARQUIVO.TXT"},
			invalid: []string{"arquivo/file.txt", "arq.txt"},
		},
		{
			name:    "spaces in pattern",
			pattern: "my file.txt",
			valid:   []string{"my file.txt", "MY FILE.TXT", "My File.Txt"},
			invalid: []string{"myfile.txt", "my-file.txt"},
		},
		{
			name:    "long pattern",
			pattern: "src/**/*.test.{js,ts}",
			valid:   []string{"src/file.test.js", "src/path/file.test.ts", "src/a/b/c.test.js"},
			invalid: []string{"src/file.js", "src/file.test.py", "test/file.test.js"},
		},

		// ====================
		// COMBINED PATTERNS (96-105)
		// ====================
		{
			name:    "star question bracket",
			pattern: "*[0-9]?.txt",
			valid:   []string{"file1a.txt", "test5X.txt", "app9_.txt", "file12.txt"},
			invalid: []string{"file.txt", "file1.txt"},
		},
		{
			name:    "doublestar brace question",
			pattern: "**/{test,main}?.js",
			valid:   []string{"test1.js", "mainX.js", "path/test2.js", "a/b/mainY.js"},
			invalid: []string{"test.js", "main.js", "other1.js"},
		},
		{
			name:    "question bracket brace",
			pattern: "?[ab]{1,2}.txt",
			valid:   []string{"xa1.txt", "yb2.txt", "1a1.txt"},
			invalid: []string{"xa3.txt", "xab1.txt", "a1.txt"},
		},
		{
			name:    "all features combined",
			pattern: "src/**/{test,spec}*[0-9]?.{js,ts}",
			valid:   []string{"src/test1a.js", "src/path/spec5X.ts", "src/a/b/test9_.js", "src/test12.js"},
			invalid: []string{"src/test.js", "src/spec1.txt"},
		},
		{
			name:    "nested braces",
			pattern: "file{.{js,ts},bak}",
			valid:   []string{"file.js", "file.ts", "filebak"},
			invalid: []string{"file.py", "file.js.bak"},
		},
		{
			name:    "bracket negation star",
			pattern: "[!0-9]*.txt",
			valid:   []string{"a.txt", "file.txt", "Xtest.txt"},
			invalid: []string{"1.txt", "5file.txt"},
		},
		{
			name:    "star bracket question",
			pattern: "*[a-z]?.log",
			valid:   []string{"testa1.log", "filemX.log", "az_.log", "test.log", "fileA.log", "test1.log", "file.log"},
			invalid: []string{},
		},
		{
			name:    "complex real world",
			pattern: "**/pkg/**/*_test.go",
			valid:   []string{"pkg/main_test.go", "src/pkg/utils/helper_test.go", "pkg/a/b/c_test.go"},
			invalid: []string{"pkg/main.go", "pkg/test_helper.go", "src/main_test.go"},
		},
		{
			name:    "multiple wildcards type",
			pattern: "*.{go,mod,sum}",
			valid:   []string{"main.go", "go.mod", "go.sum", "file.go.sum"},
			invalid: []string{"main.py", "go.mod.bak"},
		},
		{
			name:    "glob with all path types",
			pattern: "{src,pkg}\\**\\*.{go,mod}",
			valid:   []string{"src/main.go", "src/utils/file.go", "src/test.go", "pkg/go.mod"},
			invalid: []string{"lib/main.go", "src/main.py", "pkg/mod.bak", "pkg/utils/mod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex := Glob(tt.pattern)

			for _, value := range tt.valid {
				if !regex.MatchString(value) {
					t.Errorf("Glob(%q) should match %q (regex: %s)", tt.pattern, value, regex.String())
				}
			}

			for _, value := range tt.invalid {
				if regex.MatchString(value) {
					t.Errorf("Glob(%q) should NOT match %q (regex: %s)", tt.pattern, value, regex.String())
				}
			}
		})
	}
}
