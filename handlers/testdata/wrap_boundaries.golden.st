// What the wrap must not touch.
// The reflow breaks a line at 120 columns, but only when the line is code.
// Each case below is the same rule against a different kind of not-code.
// A long call inside an ignored region. The region is already how its author wants it.

PROGRAM ignored
// stformat:off
averyveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);
// stformat:on
// The rule ends at stformat:on, so the next long line is wrapped normally.
anotherveryverylongfunction(
	firstArgument := aValueWithALongName,
	secondArgument := anotherValueWithALongName,
	thirdArgument := yetAnotherValue
);
END_PROGRAM
// The short line opens the block comment, so the long line after it is comment text.

PROGRAM blockOpen
(* block
veryveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);
*)
cal(a := 1, b := 2);
END_PROGRAM
// The comment opens and closes on the long line itself, so the next line is code again.

PROGRAM blockClosed
(* a long comment that just goes on and on and on and on and on and on and on and on and on *)
veryveryverylongfunction(
	firstArgument := aValueWithALongName,
	secondArgument := anotherValueWithALongName,
	thirdArgument := yetAnotherValue
);
END_PROGRAM
// An opening delimiter inside a string literal is text, not a comment.

PROGRAM delimiterInString
cal(
	a := 1,
	b := '(* not a comment',
	c := 2,
	d := 3,
	e := 4,
	f := 5,
	g := 6,
	h := 7,
	i := 8,
	j := 9,
	k := 10,
	l := 11,
	m := 12,
	n := 13,
	o := 14,
	p := 15
);
END_PROGRAM
// A comma inside a string literal does not separate arguments.

PROGRAM commaInString
Log(id := 1, msgFmtStr := 'one, two', strArg := 'three');
END_PROGRAM
// A string literal that runs over a line break stays one token; the list the source broke open stays broken.

PROGRAM stringSpanningLines
ADSLOGSTR(
	msgCtrlMask := ADSLOG_MSGTYPE_ERROR OR ADSLOG_MSGTYPE_MSGBOX,
	msgFmtStr := 'addChild: To many subUnits,
%s',
	strArg := sArg
);
END_PROGRAM
// Over 120 columns with a literal that runs onto the next line: the call breaks, the literal is left as it is.

PROGRAM callWithMultilineString
cal(
	a := 1,
	b := 2,
	c := 3,
	d := 4,
	e := 5,
	f := 6,
	g := 7,
	h := 8,
	i := 9,
	j := 10,
	k := 11,
	l := 12,
	m := 13,
	fmt := 'first,
second'
);
END_PROGRAM
