package service

import "testing"

// B104 regression: Canvas equation images carry the LaTeX in
// data-equation-content. An apostrophe inside a double-quoted value used to
// truncate the capture, and attribute entities were escaped a second time.
func TestRewriteEquationImages_DataEquationContent(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "apostrophe inside double-quoted value",
			in:   `<img class="equation_image" src="https://canvas.example.edu/equation_images/f%27(x)" data-equation-content="f'(x)=2x">`,
			want: `<span class="math_equation_latex">\(f'(x)=2x\)</span>`,
		},
		{
			name: "attribute-encoded ampersand is decoded once, then escaped once",
			in:   `<img src="https://canvas.example.edu/equation_images/x" data-equation-content="\begin{matrix} a &amp; b \end{matrix}">`,
			want: `<span class="math_equation_latex">\(\begin{matrix} a &amp; b \end{matrix}\)</span>`,
		},
		{
			name: "sanitizer-encoded apostrophe",
			in:   `<img src="https://canvas.example.edu/equation_images/x" data-equation-content="g&#39;(t) &lt; 1">`,
			want: `<span class="math_equation_latex">\(g'(t) &lt; 1\)</span>`,
		},
		{
			name: "double quote inside single-quoted value",
			in:   `<img src='https://canvas.example.edu/equation_images/x' data-equation-content='\text{"hi"}'>`,
			want: `<span class="math_equation_latex">\(\text{"hi"}\)</span>`,
		},
		{
			name: "falls back to the URL when data-equation-content is absent",
			in:   `<img src="https://canvas.example.edu/equation_images/x%255E2">`,
			want: `<span class="math_equation_latex">\(x^2\)</span>`,
		},
		{
			name: "non-equation image untouched",
			in:   `<img src="/courses/1/files/2/download" alt="diagram">`,
			want: `<img src="/courses/1/files/2/download" alt="diagram">`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rewriteEquationImages(tc.in); got != tc.want {
				t.Errorf("rewriteEquationImages()\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}
