// A literal's text must stop where the literal stops.
// A duration that runs into the code after it swallows that code, and a string spanning lines gets a line break put inside quoted text.

// The duration payload stops at the unit. "T#0MS THEN" read as one token loses the rest of the line.
PROGRAM durationThen
IF i_risingDebounceTime <> T#0MS THEN m_risingDebounceTimer(IN := m_state, PT := i_risingDebounceTime);
IF m_risingDebounceTimer.Q THEN updateDebounceState := TRUE; END_IF END_IF
IF x > T#0S THEN y := z;
END_IF
END_PROGRAM

// The same, with the assignment and the call after the literal.
PROGRAM durationThenMore
IF t^.p > T#0MS THEN p := t^.p;
IF r <> T#0S THEN a := b := r;
IF timer.Q OR d = T#0MS  THEN c := s;
IF ts.e > T#0MS THEN x := ts.a + ts.b + ts.c;
IF ts.e > T#0MS THEN calc(IN := TRUE);
END_IF
END_PROGRAM

// A duration with spaced components is one literal, so the blank inside it is kept.
PROGRAM durationSpaced
a:=T#1h 30m 20s;
b:=LT#2d 12h;
c:=TIME#500ms;
d:=T#0MS;
END_PROGRAM

// A string spanning lines is one token, so the comma on the next line is content.
PROGRAM stringSpanningLines
log(
a := 1,
fmt := 'first,
second',
b := 2
);
END_PROGRAM