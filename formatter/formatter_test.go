package formatter

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatIDs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple program",
			in:   "PROGRAM P\nEND_PROGRAM",
			want: "PROGRAM P\nEND_PROGRAM\n",
		},
		{
			name: "keywords uppercase",
			in:   "program p\nvar\n x: bool;\nend_var\nif x then\n x:=true;\nend_if\nend_program",
			want: "PROGRAM p\nVAR\n\tx : BOOL;\nEND_VAR\n\nIF x THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "function with return type",
			in:   "FUNCTION f: REAL\n f := 1.0;\nEND_FUNCTION",
			want: "FUNCTION f : REAL\nf := 1.0;\nEND_FUNCTION\n",
		},
		{
			name: "for loop",
			in:   "PROGRAM p\nFOR i:=1 TO 10 BY 1 DO\n i:=i+1;\nEND_FOR\nEND_PROGRAM",
			want: "PROGRAM p\nFOR i := 1 TO 10 BY 1 DO\n\ti := i + 1;\nEND_FOR\nEND_PROGRAM\n",
		},
		{
			name: "while loop",
			in:   "PROGRAM p\nWHILE x=TRUE DO\n x:=FALSE;\nEND_WHILE\nEND_PROGRAM",
			want: "PROGRAM p\nWHILE x = TRUE DO\n\tx := FALSE;\nEND_WHILE\nEND_PROGRAM\n",
		},
		{
			name: "repeat until",
			in:   "PROGRAM p\nREPEAT\n x:=FALSE;\nUNTIL x END_REPEAT\nEND_PROGRAM",
			want: "PROGRAM p\nREPEAT\n\tx := FALSE;\nUNTIL x\nEND_REPEAT\nEND_PROGRAM\n",
		},
		{
			name: "case statement",
			in:   "PROGRAM p\nCASE i OF\n1,2: x:=TRUE;\n3..5: x:=FALSE;\nELSE x:=TRUE;\nEND_CASE\nEND_PROGRAM",
			want: "PROGRAM p\nCASE i OF\n\t1, 2:\n\t\tx := TRUE;\n\n\t3..5:\n\t\tx := FALSE;\n\tELSE\n\t\tx := TRUE;\nEND_CASE\nEND_PROGRAM\n",
		},
		{
			name: "operator spacing",
			in:   "PROGRAM p\nx:=1+2*3-4/5;\ny:=(a<2 and b<>3) or c>=4;\nEND_PROGRAM",
			want: "PROGRAM p\nx := 1 + 2 * 3 - 4 / 5;\ny := (a < 2 AND b <> 3) OR c >= 4;\nEND_PROGRAM\n",
		},
		{
			name: "array and string decl",
			in:   "PROGRAM p\nVAR\n arr: ARRAY[1..10] OF INT;\n s: STRING(80) := 'hi';\nEND_VAR\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\tarr : ARRAY[1..10] OF INT;\n\ts : STRING(80) := 'hi';\nEND_VAR\nEND_PROGRAM\n",
		},
		{
			name: "type struct",
			in:   "TYPE t:\nSTRUCT\n a: INT;\nEND_STRUCT\nEND_TYPE",
			want: "TYPE t :\n\tSTRUCT\n\t\ta : INT;\n\tEND_STRUCT\nEND_TYPE\n",
		},
		{
			name: "blank line between pous",
			in:   "FUNCTION f\nEND_FUNCTION\nFUNCTION g\nEND_FUNCTION",
			want: "FUNCTION f\nEND_FUNCTION\n\nFUNCTION g\nEND_FUNCTION\n",
		},
		{
			name: "method in fb",
			in:   "FUNCTION_BLOCK fb\nMETHOD m : BOOL\n m := TRUE;\nEND_METHOD\nEND_FUNCTION_BLOCK",
			want: "FUNCTION_BLOCK fb\nMETHOD m : BOOL\nm := TRUE;\nEND_METHOD\nEND_FUNCTION_BLOCK\n",
		},
		{
			name: "statement after end_if",
			in:   "PROGRAM p\nIF x THEN\n x:=TRUE;\nEND_IF y:=1;\nEND_PROGRAM",
			want: "PROGRAM p\nIF x THEN\n\tx := TRUE;\nEND_IF\n\ny := 1;\nEND_PROGRAM\n",
		},
		{
			name: "exit and return",
			in:   "PROGRAM p\nFOR i:=1 TO 10 DO\n IF x THEN\n EXIT;\n END_IF\n RETURN;\nEND_FOR\nEND_PROGRAM",
			want: "PROGRAM p\nFOR i := 1 TO 10 DO\n\tIF x THEN\n\t\tEXIT;\n\tEND_IF\n\n\tRETURN;\nEND_FOR\nEND_PROGRAM\n",
		},
		{
			name: "comments on own line",
			in:   "PROGRAM p\nx:=1; //set x\n(* block\ncomment *)\ny:=2;\nEND_PROGRAM",
			want: "PROGRAM p\nx := 1;  // set x\n(* block\ncomment *)\ny := 2;\nEND_PROGRAM\n",
		},
		{
			name: "trailing comment keeps its place",
			in:   "PROGRAM p\nVAR\nx : INT; //the counter\nEND_VAR\nx := 1; //set x\ny := 2;\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\tx : INT;  // the counter\nEND_VAR\n\nx := 1;  // set x\ny := 2;\nEND_PROGRAM\n",
		},
		{
			name: "var colon alignment removed",
			in:   "PROGRAM p\nVAR\n a : INT;\n longName : BOOL;\nEND_VAR\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\ta : INT;\n\tlongName : BOOL;\nEND_VAR\nEND_PROGRAM\n",
		},
		{
			name: "assignment alignment removed",
			in:   "PROGRAM p\n a := 1;\n longName := 2;\n m := 1;\n mn := 2;\nEND_PROGRAM",
			want: "PROGRAM p\na := 1;\nlongName := 2;\nm := 1;\nmn := 2;\nEND_PROGRAM\n",
		},
		{
			name: "wrap long call",
			in:   "PROGRAM p\ncmd.FB_Build(rDoExtending := rDoExtending, rDoRetracting := rDoRetracting, rDiAtExtendedPosition := rDiAtExtendedPosition, rDiAtRetractedPosition := rDiAtRetractedPosition, commandTimeout := T#5S);\nEND_PROGRAM",
			want: "PROGRAM p\ncmd.FB_Build(\n\trDoExtending := rDoExtending,\n\trDoRetracting := rDoRetracting,\n\trDiAtExtendedPosition := rDiAtExtendedPosition,\n\trDiAtRetractedPosition := rDiAtRetractedPosition,\n\tcommandTimeout := T#5S\n);\nEND_PROGRAM\n",
		},
		{
			name: "short call stays inline",
			in:   "PROGRAM p\ncmd.FB_Build(a := 1, b := 2);\nEND_PROGRAM",
			want: "PROGRAM p\ncmd.FB_Build(a := 1, b := 2);\nEND_PROGRAM\n",
		},
		{
			name: "wrap long if condition",
			in:   "PROGRAM p\nIF flagAlphaOne AND flagBetaTwo AND flagGammaThree AND flagDeltaFour AND flagEpsilonFive AND flagZetaSix AND flagEtaSeven AND flagThetaEight AND flagIotaNine THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF flagAlphaOne\n\tAND flagBetaTwo\n\tAND flagGammaThree\n\tAND flagDeltaFour\n\tAND flagEpsilonFive\n\tAND flagZetaSix\n\tAND flagEtaSeven\n\tAND flagThetaEight\n\tAND flagIotaNine\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "combined condition stays together, elsif wraps",
			in:   "PROGRAM p\nIF (stateIsRunning OR stateIsStarting) AND inputSignal.Valid AND outputSignal.Enable AND machineStatus.Ready AND operatorGate.Closed THEN\n    x := TRUE;\nELSIF backupFlag AND standbyFlag AND cascadeMode AND recoveryFlag AND notificationPresent AND auxiliaryHold AND brakeReleased THEN\n    x := FALSE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tstateIsRunning\n\t\tOR stateIsStarting\n\t)\n\tAND inputSignal.Valid\n\tAND outputSignal.Enable\n\tAND machineStatus.Ready\n\tAND operatorGate.Closed\nTHEN\n\tx := TRUE;\nELSIF backupFlag\n\tAND standbyFlag\n\tAND cascadeMode\n\tAND recoveryFlag\n\tAND notificationPresent\n\tAND auxiliaryHold\n\tAND brakeReleased\nTHEN\n\tx := FALSE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "overlong combined condition deep splits",
			in:   "PROGRAM p\nIF (conditionAlphaThatIsVeryLong AND conditionBetaThatIsVeryLong AND conditionGammaThatIsVeryLong AND conditionDeltaThatIsVeryLong AND conditionEpsilonThatIsVeryLong AND conditionZetaThatIsVeryLong) AND simpleFlag THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tconditionAlphaThatIsVeryLong\n\t\tAND conditionBetaThatIsVeryLong\n\t\tAND conditionGammaThatIsVeryLong\n\t\tAND conditionDeltaThatIsVeryLong\n\t\tAND conditionEpsilonThatIsVeryLong\n\t\tAND conditionZetaThatIsVeryLong\n\t)\n\tAND simpleFlag\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "short if stays inline",
			in:   "PROGRAM p\nIF a AND b THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF a AND b THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "string literal with then and and keywords is not split",
			in:   "PROGRAM p\nIF commandText = 'FIND THEN AND OR XOR' THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF commandText = 'FIND THEN AND OR XOR' THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "mixed and/or with two short paren groups",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride) OR (flagEmergencyStopActive AND flagPowerAvailable) AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t)\n\tOR (\n\t\tflagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t)\n\tAND flagGateClosed\n\tAND flagInterlockEngaged\n\tAND flagSequenceRunning\n\tAND flagAbortRequested\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "overlong first paren group deep splits second stays whole",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged) OR (flagAbortRequested AND flagResetPermitted) THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t\tAND flagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t\tAND flagGateClosed\n\t\tAND flagInterlockEngaged\n\t)\n\tOR (\n\t\tflagAbortRequested\n\t\tAND flagResetPermitted\n\t)\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "both paren groups overlong deep split at two indent levels",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged) OR (flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete AND flagAutoModeSelected AND flagManualModeSelected) THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t\tAND flagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t\tAND flagGateClosed\n\t\tAND flagInterlockEngaged\n\t)\n\tOR (\n\t\tflagSequenceRunning\n\t\tAND flagAbortRequested\n\t\tAND flagResetPermitted\n\t\tAND flagCycleComplete\n\t\tAND flagAutoModeSelected\n\t\tAND flagManualModeSelected\n\t)\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "nested parens deep split at three indent levels",
			in:   "PROGRAM p\nIF ((flagOperationalReady OR flagMaintenanceOverride) AND (flagEmergencyStopActive OR flagPowerAvailable)) AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\t(\n\t\t\tflagOperationalReady\n\t\t\tOR flagMaintenanceOverride\n\t\t)\n\t\tAND (\n\t\t\tflagEmergencyStopActive\n\t\t\tOR flagPowerAvailable\n\t\t)\n\t)\n\tAND flagGateClosed\n\tAND flagInterlockEngaged\n\tAND flagSequenceRunning\n\tAND flagAbortRequested\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "mixed top-level paren groups with trailing chain",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride) OR (flagEmergencyStopActive AND flagPowerAvailable) OR (flagGateClosed AND flagInterlockEngaged) AND flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t)\n\tOR (\n\t\tflagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t)\n\tOR (\n\t\tflagGateClosed\n\t\tAND flagInterlockEngaged\n\t)\n\tAND flagSequenceRunning\n\tAND flagAbortRequested\n\tAND flagResetPermitted\n\tAND flagCycleComplete\nTHEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "duration literal does not swallow the rest of the line",
			in:   "PROGRAM p\nIF i_risingDebounceTime <> T#0MS THEN m_risingDebounceTimer(IN := m_state, PT := i_risingDebounceTime);\nIF m_risingDebounceTimer.Q THEN updateDebounceState := TRUE; END_IF END_IF\nIF x > T#0S THEN y := z;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF i_risingDebounceTime <> T#0MS THEN\n\tm_risingDebounceTimer(IN := m_state, PT := i_risingDebounceTime);\n\tIF m_risingDebounceTimer.Q THEN\n\t\tupdateDebounceState := TRUE;\n\tEND_IF\nEND_IF\n\nIF x > T#0S THEN\n\ty := z;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "duration with spaced components stays one literal",
			in:   "PROGRAM p\na:=T#1h 30m 20s;\nb:=LT#2d 12h;\nc:=TIME#500ms;\nd:=T#0MS;\nEND_PROGRAM",
			want: "PROGRAM p\na := T#1h 30m 20s;\nb := LT#2d 12h;\nc := TIME#500ms;\nd := T#0MS;\nEND_PROGRAM\n",
		},
		{
			name: "string literal spanning lines is kept verbatim",
			in:   "PROGRAM p\nADSLOGSTR(\nmsgCtrlMask := ADSLOG_MSGTYPE_ERROR OR ADSLOG_MSGTYPE_MSGBOX,\nmsgFmtStr := 'addChild: To many subUnits,\n%s',\nstrArg := sArg\n);\nEND_PROGRAM",
			want: "PROGRAM p\nADSLOGSTR(msgCtrlMask := ADSLOG_MSGTYPE_ERROR OR ADSLOG_MSGTYPE_MSGBOX, msgFmtStr := 'addChild: To many subUnits,\n%s', strArg := sArg);\nEND_PROGRAM\n",
		},
		{
			// The line is over 120 characters, but reflowing it would rewrite
			// the literal that continues on the next line.
			name: "overlong call with a multiline string argument is not reflowed",
			in:   "PROGRAM p\ncal(a := 1, b := 2, c := 3, d := 4, e := 5, f := 6, g := 7, h := 8, i := 9, j := 10, k := 11, l := 12, m := 13, fmt := 'first,\nsecond');\nEND_PROGRAM",
			want: "PROGRAM p\ncal(a := 1, b := 2, c := 3, d := 4, e := 5, f := 6, g := 7, h := 8, i := 9, j := 10, k := 11, l := 12, m := 13, fmt := 'first,\nsecond');\nEND_PROGRAM\n",
		},
		{
			name: "comma inside a string does not split call arguments",
			in:   "PROGRAM p\nLog(id := 1, msgFmtStr := 'one, two', strArg := 'three');\nEND_PROGRAM",
			want: "PROGRAM p\nLog(id := 1, msgFmtStr := 'one, two', strArg := 'three');\nEND_PROGRAM\n",
		},
		{
			name: "trailing comment does not count towards the line length",
			in:   "PROGRAM p\nsomeFunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue); // an explanation of the call\nEND_PROGRAM",
			want: "PROGRAM p\nsomeFunction(\n\tfirstArgument := aValueWithALongName,\n\tsecondArgument := anotherValueWithALongName,\n\tthirdArgument := yetAnotherValue\n);  // an explanation of the call\nEND_PROGRAM\n",
		},
		{
			name: "trailing comment moves to the then line when the condition wraps",
			in:   "PROGRAM p\nIF flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete AND flagAutoModeSelected THEN // all flags\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF flagOperationalReady\n\tAND flagMaintenanceOverride\n\tAND flagEmergencyStopActive\n\tAND flagPowerAvailable\n\tAND flagGateClosed\n\tAND flagInterlockEngaged\n\tAND flagSequenceRunning\n\tAND flagAbortRequested\n\tAND flagResetPermitted\n\tAND flagCycleComplete\n\tAND flagAutoModeSelected\nTHEN  // all flags\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "stformat off and on ignore a section",
			in:   "PROGRAM p\n// stformat:off\nVAR\n    ugly    :   INT;\nEND_VAR\n// stformat:on\nIF a > 0 THEN\n// stformat:off\n    weird    (  1,2 )  ;\n// stformat:on\n    b := a;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\n// stformat:off\nVAR\n    ugly    :   INT;\nEND_VAR\n// stformat:on\nIF a > 0 THEN\n\t// stformat:off\n    weird    (  1,2 )  ;\n\t// stformat:on\n\tb := a;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "ignored section is not wrapped",
			in:   "PROGRAM p\n// stformat:off\naveryveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);\na := 1;\nlongerName := 2;\n// stformat:on\nEND_PROGRAM",
			want: "PROGRAM p\n// stformat:off\naveryveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);\na := 1;\nlongerName := 2;\n// stformat:on\nEND_PROGRAM\n",
		},
		{
			// The short line opens the comment, so a later long line is inside
			// it and must stay as it is.
			name: "call inside a block comment opened on a short line is not wrapped",
			in:   "PROGRAM p\n(* block\nveryveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);\n*)\ncal(a := 1, b := 2);\nEND_PROGRAM",
			want: "PROGRAM p\n(* block\nveryveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);\n*)\ncal(a := 1, b := 2);\nEND_PROGRAM\n",
		},
		{
			// The comment opens and closes on the long line itself, so the
			// line after it is back outside a comment and still wrappable.
			name: "line after a self-closed block comment is wrapped",
			in:   "PROGRAM p\n(* a long comment that just goes on and on and on and on and on and on and on and on and on *)\nveryveryverylongfunction(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue);\nEND_PROGRAM",
			want: "PROGRAM p\n(* a long comment that just goes on and on and on and on and on and on and on and on and on *)\nveryveryverylongfunction(\n\tfirstArgument := aValueWithALongName,\n\tsecondArgument := anotherValueWithALongName,\n\tthirdArgument := yetAnotherValue\n);\nEND_PROGRAM\n",
		},
		{
			name: "comment delimiter inside a string literal does not block wrapping",
			in:   "PROGRAM p\ncal(a := 1, b := '(* not a comment', c := 2, d := 3, e := 4, f := 5, g := 6, h := 7, i := 8, j := 9, k := 10, l := 11, m := 12, n := 13, o := 14, p := 15);\nEND_PROGRAM",
			want: "PROGRAM p\ncal(\n\ta := 1,\n\tb := '(* not a comment',\n\tc := 2,\n\td := 3,\n\te := 4,\n\tf := 5,\n\tg := 6,\n\th := 7,\n\ti := 8,\n\tj := 9,\n\tk := 10,\n\tl := 11,\n\tm := 12,\n\tn := 13,\n\to := 14,\n\tp := 15\n);\nEND_PROGRAM\n",
		},
		{
			name: "ignored section survives two passes",
			in:   "PROGRAM p\n// stformat:off\nVAR\n    ugly    :   INT;\nEND_VAR\n// stformat:on\nx:=1;\nEND_PROGRAM",
			want: "PROGRAM p\n// stformat:off\nVAR\n    ugly    :   INT;\nEND_VAR\n// stformat:on\nx := 1;\nEND_PROGRAM\n",
		},
		{
			name: "directive trailing code stays on that line",
			in:   "PROGRAM p\nIF a > 0 THEN\nx := 1; // stformat:off\n   y   :=   2;\n// stformat:on\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF a > 0 THEN\n\tx := 1;  // stformat:off\n   y   :=   2;\n\t// stformat:on\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "stformat ignore in the leading comment block leaves the file alone",
			in:   "// stformat:ignore\nPROGRAM p\nx:=1;\nEND_PROGRAM",
			want: "// stformat:ignore\nPROGRAM p\nx:=1;\nEND_PROGRAM",
		},
		{
			name: "stformat ignore after code is an ordinary comment",
			in:   "PROGRAM p\nx:=1;\n// stformat:ignore\ny:=2;\nEND_PROGRAM",
			want: "PROGRAM p\nx := 1;\n// stformat:ignore\ny := 2;\nEND_PROGRAM\n",
		},
		{
			name: "directive spelling and case are preserved",
			in:   "PROGRAM p\nx:=1; // STFormat : Off\n    y   :=   2;\n// stformat:on\nz:=3;\nEND_PROGRAM",
			want: "PROGRAM p\nx := 1;  // STFormat : Off\n    y   :=   2;\n// stformat:on\nz := 3;\nEND_PROGRAM\n",
		},
		{
			// A typed literal runs from its type name to its closing quote,
			// whichever quote it uses. Reading only up to the opening one leaves
			// the text behind to be lexed as code.
			name: "typed string and character literals keep their payload",
			in:   "PROGRAM p\nVAR\n a: string := STRING#'typed';\n b: wstring := STRING#\"typed\";\n c: char := CHAR#'a';\n d: wchar := WCHAR#\"ab\";\nEND_VAR\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\ta : STRING := STRING#'typed';\n\tb : WSTRING := STRING#\"typed\";\n\tc : CHAR := CHAR#'a';\n\td : WCHAR := WCHAR#\"ab\";\nEND_VAR\nEND_PROGRAM\n",
		},
		{
			// A doubled quote is a quote in the text, not the end of the
			// literal. Stopping at the first one puts a blank inside quoted text.
			name: "doubled quote inside a literal is not the end of it",
			in:   "PROGRAM p\nVAR\n a: string := 'don''t';\n b: wstring := \"say \"\"hi\"\"\";\n c: string := STRING#'it''s';\nEND_VAR\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\ta : STRING := 'don''t';\n\tb : WSTRING := \"say \"\"hi\"\"\";\n\tc : STRING := STRING#'it''s';\nEND_VAR\nEND_PROGRAM\n",
		},
		{
			// The output bindings are operators rather than keywords, and both
			// S= and s= are accepted, so the spelling of the source is kept.
			name: "output bindings keep the case they were written with",
			in:   "PROGRAM p\nVAR\n b: bool;\nEND_VAR\nb s= TRUE;\nb r= FALSE;\nb ref= b;\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\tb : BOOL;\nEND_VAR\n\nb s= TRUE;\nb r= FALSE;\nb ref= b;\nEND_PROGRAM\n",
		},
		{
			// END_PROGRAM ends no statement, so nothing before the comment
			// asked for a line break and the comment would be glued to it.
			name: "comment on its own line after a block end keeps its line",
			in:   "PROGRAM p\nx:=1;\nEND_PROGRAM\n// a comment about the program\nFUNCTION f\nEND_FUNCTION",
			want: "PROGRAM p\nx := 1;\nEND_PROGRAM\n// a comment about the program\n\nFUNCTION f\nEND_FUNCTION\n",
		},
		{
			// A TYPE body is a list of values like a call's argument list, so
			// it breaks at the same limit into the same layout.
			name: "long type value list breaks one member per line",
			in:   "TYPE states :\n(A := 0, B := 1, C := 2, D := 3, E := 4, F := 5, G := 6, H := 7, I := 8, J := 9, K := 10, L := 11, M := 12, N := 13, O := 14, P := 15);\nEND_TYPE",
			want: "TYPE states :\n\t(\n\t\tA := 0,\n\t\tB := 1,\n\t\tC := 2,\n\t\tD := 3,\n\t\tE := 4,\n\t\tF := 5,\n\t\tG := 6,\n\t\tH := 7,\n\t\tI := 8,\n\t\tJ := 9,\n\t\tK := 10,\n\t\tL := 11,\n\t\tM := 12,\n\t\tN := 13,\n\t\tO := 14,\n\t\tP := 15\n\t);\nEND_TYPE\n",
		},
		{
			// The list was opened on the line above, which carries the comment.
			// The bracket stays where it is and only the members break.
			name: "type value list continued from the line above breaks its members",
			in:   "TYPE continued :\n(  // the names\n\tA := 0, B := 1, C := 2, D := 3, E := 4, F := 5, G := 6, H := 7, I := 8, J := 9, K := 10, L := 11, M := 12, N := 13, O := 14, P := 15\n);\nEND_TYPE",
			want: "TYPE continued :\n\t(  // the names\n\t\tA := 0,\n\t\tB := 1,\n\t\tC := 2,\n\t\tD := 3,\n\t\tE := 4,\n\t\tF := 5,\n\t\tG := 6,\n\t\tH := 7,\n\t\tI := 8,\n\t\tJ := 9,\n\t\tK := 10,\n\t\tL := 11,\n\t\tM := 12,\n\t\tN := 13,\n\t\tO := 14,\n\t\tP := 15\n\t);\nEND_TYPE\n",
		},
		{
			// The line is over the limit, but the ')' it ends with closes
			// nothing: without a level carried over from an earlier line there
			// is no list to break up.
			name: "stray closing bracket with no level open is not a list",
			in:   "PROGRAM q\ny := aaaaaaaaaaaaaaaaaaaaaa + bbbbbbbbbbbbbbbbbbbbbbbb + cccccccccccccccccccccccc + dddddddddddddddddddddddd + eeeee);\nEND_PROGRAM",
			want: "PROGRAM q\ny := aaaaaaaaaaaaaaaaaaaaaa + bbbbbbbbbbbbbbbbbbbbbbbb + cccccccccccccccccccccccc + dddddddddddddddddddddddd + eeeee);\nEND_PROGRAM\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := Format(c.in); got != c.want {
				t.Errorf("Format() mismatch\n got: %q\nwant: %q", got, c.want)
			}
		})
	}
}

func TestIdempotent(t *testing.T) {
	t.Parallel()
	in := "function_block fb\nvar\nx: bool:=true;\nend_var\nif x then\nx:=false;\nend_if\nend_function_block"
	once := Format(in)
	twice := Format(once)
	if once != twice {
		t.Errorf("formatter not idempotent\nonce:  %q\ntwice: %q", once, twice)
	}
}

// TestIdempotentRegression checks that a second pass never changes the output.
// The shapes here are the ones that are easy to get wrong: a duration literal
// whose payload can run into the code that follows it, and a string literal
// spanning lines whose comma looks like an argument separator. Names are kept
// to single letters so the inputs stay readable; a case that needs a long line
// says so with wraps, so shortening can never quietly stop exercising the
// wrapping.
func TestIdempotentRegression(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		// wraps marks an input that must come out multi-line, so the case
		// keeps covering the wrapping code it was written for.
		wraps bool
	}{
		{
			name: "duration literal before call",
			in: "PROGRAM p\n" +
				"IF t <> T#0MS THEN timer(IN := s, PT := t);\n" +
				"IF timer.Q THEN s := TRUE; END_IF END_IF\n" +
				"IF t <> T#0MS THEN timer(IN := NOT s, PT := t);\n" +
				"END_IF\nEND_PROGRAM\n",
		},
		{
			name: "duration literal before assignment",
			in: "PROGRAM p\n" +
				"IF t^.p > T#0MS THEN p := t^.p;\n" +
				"IF r <> T#0S THEN a := b := r;\n" +
				"IF timer.Q OR d = T#0MS  THEN c := s;\n" +
				"IF ts.e > T#0MS THEN x := ts.a + ts.b + ts.c;\n" +
				"IF ts.e > T#0MS THEN calc(IN := TRUE);\n" +
				"END_PROGRAM\n",
		},
		{
			name: "multiline string argument",
			in: "PROGRAM p\n" +
				"log(\n" +
				"a := 1,\n" +
				"fmt := 'first,\n" +
				"second',\n" +
				"b := 2\n" +
				");\n" +
				"END_PROGRAM\n",
		},
		{
			name: "wrapped condition with then",
			in: "PROGRAM p\n" +
				"IF a AND b AND c AND d AND e AND f AND g AND h AND i AND j AND k AND l AND m AND n AND o AND p AND q AND r AND s AND t AND u THEN\n" +
				"x := TRUE;\n" +
				"END_IF\n" +
				"END_PROGRAM\n",
			wraps: true,
		},
		{
			name: "trailing comment on a wrapped call",
			in: "PROGRAM p\n" +
				"cal(a := 1, b := 2, c := 3, d := 4, e := 5, f := 6, g := 7, h := 8, i := 9, j := 10, k := 11, l := 12, m := 13, n := 14, o := 15); // why\n" +
				"END_PROGRAM\n",
			wraps: true,
		},
		{
			name: "ignored section",
			in: "PROGRAM p\n" +
				"// stformat:off\nVAR\n    ugly    :   INT;\nEND_VAR\n// stformat:on\nIF a > 0 THEN b := a; END_IF\nEND_PROGRAM\n",
		},
		{
			// The closing quote of a typed literal used to be missed, which
			// left the rest of the file to be lexed as string content.
			name: "typed literals",
			in: "PROGRAM p\n" +
				"VAR\n a: string := STRING#'x';\n b: string := STRING#\"y\";\n c: char := CHAR#'z';\nEND_VAR\n" +
				"a := 'x';\nEND_PROGRAM\n",
		},
		{
			name: "value list continued from the line above",
			in: "TYPE continued :\n" +
				"(  // the names\n" +
				"\tA := 0, B := 1, C := 2, D := 3, E := 4, F := 5, G := 6, H := 7, I := 8, J := 9, K := 10, L := 11, M := 12, N := 13, O := 14, P := 15\n" +
				");\nEND_TYPE\n",
			wraps: true,
		},
		{
			name: "comment between two pous",
			in: "PROGRAM p\n" +
				"x:=1;\nEND_PROGRAM\n" +
				"// about the next one\n" +
				"FUNCTION f\nEND_FUNCTION\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			once := Format(c.in)
			twice := Format(once)
			if once != twice {
				t.Errorf("second pass changed the output\nonce:  %q\ntwice: %q", once, twice)
			}
			if c.wraps && !strings.Contains(once, "\n\t") {
				t.Errorf("input no longer wraps, so it no longer covers wrapping:\n%q", once)
			}
		})
	}
}

// TestCopyStream checks that the stream entry point produces the same output as
// Format, that a file-level ignore is streamed through unparsed, and that the
// directives still work when the input arrives from a reader.
func TestCopyStream(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"plain":                    "PROGRAM p\nx:=1;\nEND_PROGRAM\n",
		"section off":              "PROGRAM p\n// stformat:off\n   y  :=  2;\n// stformat:on\nz:=3;\nEND_PROGRAM\n",
		"block comment directive":  "PROGRAM p\n(* stformat:off *)\n   y  :=  2;\n(* stformat:on *)\nz:=3;\nEND_PROGRAM\n",
		"leading ignore":           "// stformat:ignore\nPROGRAM p\n   x:=1;\nEND_PROGRAM",
		"ignore after code":        "PROGRAM p\nx:=1;\n// stformat:ignore\ny:=2;\nEND_PROGRAM",
		"leading blank line":       "\n// stformat:ignore\nPROGRAM p\n   x:=1;\nEND_PROGRAM",
		"leading block comment":    "(* header *)\n// stformat:ignore\nPROGRAM p\n   x:=1;\nEND_PROGRAM",
		"multi line block comment": "(* stformat:ignore\n   still leading\n*)\nPROGRAM p\n   x:=1;\nEND_PROGRAM",
		"ignore then code":         "// stformat:ignore\nx := 1;\n// stformat:on\ny:=2;\n",
		"no trailing newline":      "PROGRAM p\nx:=1;",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if _, err := Copy(&buf, strings.NewReader(in)); err != nil {
				t.Fatalf("Copy: %v", err)
			}
			if want := Format(in); buf.String() != want {
				t.Errorf("Copy()\n got: %q\nwant: %q", buf.String(), want)
			}
			// The stream must also be stable under a second pass.
			var again bytes.Buffer
			if _, err := Copy(&again, strings.NewReader(buf.String())); err != nil {
				t.Fatalf("Copy: %v", err)
			}
			if again.String() != buf.String() {
				t.Errorf("second Copy changed the output\n once:  %q\n twice: %q", buf.String(), again.String())
			}
		})
	}
}

// TestCopyUnterminatedInput guards the stream against losing a last line that
// has no line break. When every line before it is a comment or blank, the
// leading-comment scan reaches the end of the input, and that line must still
// be part of what gets formatted.
func TestCopyUnterminatedInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{in: "x:=1;", want: "x := 1;\n"},
		{in: "//hello", want: "// hello\n"},
		{in: "// header\n\nx:=1;", want: "// header\nx := 1;\n"},
		{in: "(* a\nb *)\nx:=1;", want: "(* a\nb *)\nx := 1;\n"},
		{in: "// stformat:ignore", want: "// stformat:ignore"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		if _, err := Copy(&buf, strings.NewReader(c.in)); err != nil {
			t.Fatalf("Copy(%q): %v", c.in, err)
		}
		if buf.String() != c.want {
			t.Errorf("Copy(%q) = %q, want %q", c.in, buf.String(), c.want)
		}
	}
}

// TestByteOrderMark checks that a UTF-8 BOM survives formatting, in front of
// the output, and does not get in the way of a leading directive.
func TestByteOrderMark(t *testing.T) {
	t.Parallel()
	if got, want := Format(utf8BOM+"x:=1;\n"), utf8BOM+"x := 1;\n"; got != want {
		t.Errorf("Format with BOM = %q, want %q", got, want)
	}
	ignored := utf8BOM + "// stformat:ignore\nx:=1;\n"
	if got := Format(ignored); got != ignored {
		t.Errorf("Format ignored file with BOM = %q, want it unchanged", got)
	}
	var buf bytes.Buffer
	if _, err := Copy(&buf, strings.NewReader(ignored)); err != nil {
		t.Fatal(err)
	}
	if buf.String() != ignored {
		t.Errorf("Copy ignored file with BOM = %q, want it unchanged", buf.String())
	}
}

// TestLineEndings checks that the output keeps the line ending of the input: a
// CRLF file stays CRLF and an LF file stays LF, so formatting never turns a
// file into a mixed-ending one or rewrites every line of it.
func TestLineEndings(t *testing.T) {
	t.Parallel()
	body := "PROGRAM p\nVAR\n x : INT; //the counter\nEND_VAR\n(* block\ncomment *)\nx:=1; //set x\nEND_PROGRAM\n"
	wantBody := "PROGRAM p\nVAR\n\tx : INT;  // the counter\nEND_VAR\n\n(* block\ncomment *)\nx := 1;  // set x\nEND_PROGRAM\n"

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "lf stays lf", in: body, want: wantBody},
		{
			name: "crlf stays crlf",
			in:   strings.ReplaceAll(body, "\n", "\r\n"),
			want: strings.ReplaceAll(wantBody, "\n", "\r\n"),
		},
		{
			name: "ignored region gets the input ending too",
			in:   "PROGRAM p\r\n// stformat:off\r\n   y  :=  2;\r\n// stformat:on\r\nz:=3;\r\nEND_PROGRAM\r\n",
			want: "PROGRAM p\r\n// stformat:off\r\n   y  :=  2;\r\n// stformat:on\r\nz := 3;\r\nEND_PROGRAM\r\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Format(c.in)
			if got != c.want {
				t.Errorf("Format()\n got: %q\nwant: %q", got, c.want)
			}
			if again := Format(got); again != got {
				t.Errorf("not idempotent\nonce: %q\ntwice: %q", got, again)
			}
		})
	}
}

// TestLineEnding checks the detection of the input's line ending.
func TestLineEnding(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: "\n"},
		{in: "no line breaks", want: "\n"},
		{in: "a\nb\n", want: "\n"},
		{in: "a\r\nb\r\n", want: "\r\n"},
		{in: "a\r\nb\r\nc\n", want: "\r\n"},
		{in: "a\nb\nc\r\n", want: "\n"},
		{in: "a\r\nb\n", want: "\n"},
	}
	for _, c := range cases {
		if got := LineEnding(c.in); got != c.want {
			t.Errorf("LineEnding(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestFormatWith checks that the caller can impose the line ending, which the
// XML handler needs because a single-line CDATA block has no ending of its own
// and must follow the rest of the file.
func TestFormatWith(t *testing.T) {
	t.Parallel()
	source := "PROGRAM p\nx:=1;\ny:=2;\nEND_PROGRAM\n"
	if got, want := FormatWith(source, "\r\n"), "PROGRAM p\r\nx := 1;\r\ny := 2;\r\nEND_PROGRAM\r\n"; got != want {
		t.Errorf("FormatWith CRLF\n got: %q\nwant: %q", got, want)
	}
	if got, want := FormatWith(source, "\n"), Format(source); got != want {
		t.Errorf("FormatWith LF\n got: %q\nwant: %q", got, want)
	}
	// An unusable ending falls back to the one in the source.
	crlf := strings.ReplaceAll(source, "\n", "\r\n")
	if got, want := FormatWith(crlf, "\r"), Format(crlf); got != want {
		t.Errorf("FormatWith bad ending\n got: %q\nwant: %q", got, want)
	}
}

func TestNormalizeComment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "line comment gets a space", in: "//set x", want: "// set x"},
		{name: "case is kept", in: "//Set", want: "// Set"},
		{name: "line comment already spaced", in: "// set x", want: "// set x"},
		{name: "indentation inside comment kept", in: "//   indented", want: "//   indented"},
		{name: "commented-out code keeps its case", in: "//x := 1;", want: "// x := 1;"},
		{name: "doc comment untouched", in: "///doc", want: "///doc"},
		{name: "rule untouched", in: "//////////", want: "//////////"},
		{name: "dashes untouched", in: "//---", want: "//---"},
		{name: "non-ascii word gets a space", in: "//Änderung", want: "// Änderung"},
		{name: "empty line comment", in: "//", want: "//"},
		{name: "whitespace line comment", in: "// ", want: "//"},
		{name: "block comment gets a space", in: "(*block*)", want: "(* block*)"},
		{name: "block comment already spaced", in: "(* block *)", want: "(* block *)"},
		{name: "multiline block comment", in: "(* block\ncomment *)", want: "(* block\ncomment *)"},
		{name: "banner untouched", in: "(*****************)", want: "(*****************)"},
		{name: "short banner untouched", in: "(*****)", want: "(*****)"},
		{name: "pragma unchanged", in: "{attribute 'xl' := true}", want: "{attribute 'xl' := true}"},
		{name: "empty block comment", in: "(* *)", want: "(* *)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeComment(c.in); got != c.want {
				t.Errorf("normalizeComment(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestFormatFixes pins the layout of constructs that used to be mangled.
func TestFormatFixes(t *testing.T) {
	t.Parallel()
	// A call whose code fits in 120 bytes but, behind one tab of indentation,
	// takes more than 120 columns.
	call := "foo(" + strings.Repeat("a", 55) + ", " + strings.Repeat("b", 55) + ");"
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "short date literals",
			in:   "d:=D#2020-01-01; e:=ld#2020-01-01;",
			want: "d := D#2020-01-01;\ne := ld#2020-01-01;\n",
		},
		{
			name: "second off inside an ignored section is a comment",
			in:   "// stformat:off\na:=1;\n// stformat:off\nb:=2;\n// stformat:on\nc:=3;\n",
			want: "// stformat:off\na:=1;\n// stformat:off\nb:=2;\n// stformat:on\nc := 3;\n",
		},
		{
			name: "on outside an ignored section is a comment",
			in:   "// stformat:off\na:=1;\n// stformat:on\nb:=2;\n// stformat:on\nc:=3;\n",
			want: "// stformat:off\na:=1;\n// stformat:on\nb := 2;\n// stformat:on\nc := 3;\n",
		},
		{
			name: "jump label",
			in:   "PROGRAM p\nJMP lbl;\nlbl : x:=1;\nEND_PROGRAM",
			want: "PROGRAM p\nJMP lbl;\nlbl: x := 1;\nEND_PROGRAM\n",
		},
		{
			name: "declaration colon is not a label",
			in:   "PROGRAM p\nVAR\nx:INT;\nEND_VAR\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\tx : INT;\nEND_VAR\nEND_PROGRAM\n",
		},
		{
			name: "NOT with parentheses",
			in:   "x:=NOT (a) AND NOT b;",
			want: "x := NOT(a) AND NOT b;\n",
		},
		{
			name: "repetition in an array initialiser",
			in:   "VAR\narr : ARRAY[1..10] OF INT := [10 (0)];\nEND_VAR",
			want: "VAR\n\tarr : ARRAY[1..10] OF INT := [10(0)];\nEND_VAR\n",
		},
		{
			name: "line width counts the tab as four columns",
			in:   "IF x THEN\n" + call + "\nEND_IF",
			want: "IF x THEN\n\tfoo(\n\t\t" + strings.Repeat("a", 55) + ",\n\t\t" + strings.Repeat("b", 55) + "\n\t);\nEND_IF\n",
		},
		{
			name: "line width counts characters, not bytes",
			in:   "s := '" + strings.Repeat("ä", 100) + "';",
			want: "s := '" + strings.Repeat("ä", 100) + "';\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Format(c.in)
			if got != c.want {
				t.Errorf("Format()\n got: %q\nwant: %q", got, c.want)
			}
			if again := Format(got); again != got {
				t.Errorf("not idempotent\n once: %q\ntwice: %q", got, again)
			}
		})
	}
}

// TestVerify checks the safety net: whitespace, keyword case and comment
// spacing may change, anything else is refused.
func TestVerify(t *testing.T) {
	t.Parallel()
	ok := [][2]string{
		{"x:=1;", "x := 1;\n"},
		{"if a then b; end_if", "IF a THEN\n\tb;\nEND_IF\n"},
		{"//note", "// note"},
		{"a := 1;\r\nb := 2;\r\n", "a := 1;\nb := 2;\n"},
		{utf8BOM + "x:=1;", utf8BOM + "x := 1;\n"},
		{"s := '\r\r\n';", "s := '\r\n';"},
	}
	for _, c := range ok {
		if err := Verify(c[0], c[1]); err != nil {
			t.Errorf("Verify(%q, %q) = %v, want nil", c[0], c[1], err)
		}
	}
	bad := [][2]string{
		{"x := 1;", "x := 2;"},                 // literal changed
		{"x := y;", "x := Y;"},                 // identifier case is not a keyword's
		{"x := 'a';", "x := 'A';"},             // string content
		{"x := y ? 1;", "x := y 1;"},           // dropped character
		{"x := 1;", "x := 1;;"},                // added token
		{"// Note", "// note"},                 // comment text
		{"{attribute 'a'}", "{attribute 'b'}"}, // pragma text
		{utf8BOM + "x := 1;", "x := 1;"},       // byte order mark dropped
	}
	for _, c := range bad {
		if err := Verify(c[0], c[1]); err == nil {
			t.Errorf("Verify(%q, %q) = nil, want an error", c[0], c[1])
		}
	}
}
