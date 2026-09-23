package formatter

import "testing"

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
			want: "PROGRAM p\nVAR\n\tarr : ARRAY[1..10] OF INT;\n\ts   : STRING(80) := 'hi';\nEND_VAR\nEND_PROGRAM\n",
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
			in:   "PROGRAM p\nx:=1; // comment\n(* block *)\ny:=2;\nEND_PROGRAM",
			want: "PROGRAM p\nx := 1;\n// Comment\n(* Block *)\ny := 2;\nEND_PROGRAM\n",
		},
		{
			name: "var colon alignment",
			in:   "PROGRAM p\nVAR\n a : INT;\n longName : BOOL;\nEND_VAR\nEND_PROGRAM",
			want: "PROGRAM p\nVAR\n\ta        : INT;\n\tlongName : BOOL;\nEND_VAR\nEND_PROGRAM\n",
		},
		{
			name: "assignment alignment",
			in:   "PROGRAM p\n a := 1;\n longName := 2;\nEND_PROGRAM",
			want: "PROGRAM p\na        := 1;\nlongName := 2;\nEND_PROGRAM\n",
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
			want: "PROGRAM p\nIF flagAlphaOne\n\tAND flagBetaTwo\n\tAND flagGammaThree\n\tAND flagDeltaFour\n\tAND flagEpsilonFive\n\tAND flagZetaSix\n\tAND flagEtaSeven\n\tAND flagThetaEight\n\tAND flagIotaNine THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "combined condition stays together, elsif wraps",
			in:   "PROGRAM p\nIF (stateIsRunning OR stateIsStarting) AND inputSignal.Valid AND outputSignal.Enable AND machineStatus.Ready AND operatorGate.Closed THEN\n    x := TRUE;\nELSIF backupFlag AND standbyFlag AND cascadeMode AND recoveryFlag AND notificationPresent AND auxiliaryHold AND brakeReleased THEN\n    x := FALSE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tstateIsRunning\n\t\tOR stateIsStarting\n\t)\n\tAND inputSignal.Valid\n\tAND outputSignal.Enable\n\tAND machineStatus.Ready\n\tAND operatorGate.Closed THEN\n\tx := TRUE;\nELSIF backupFlag\n\tAND standbyFlag\n\tAND cascadeMode\n\tAND recoveryFlag\n\tAND notificationPresent\n\tAND auxiliaryHold\n\tAND brakeReleased THEN\n\tx := FALSE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "overlong combined condition deep splits",
			in:   "PROGRAM p\nIF (conditionAlphaThatIsVeryLong AND conditionBetaThatIsVeryLong AND conditionGammaThatIsVeryLong AND conditionDeltaThatIsVeryLong AND conditionEpsilonThatIsVeryLong AND conditionZetaThatIsVeryLong) AND simpleFlag THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tconditionAlphaThatIsVeryLong\n\t\tAND conditionBetaThatIsVeryLong\n\t\tAND conditionGammaThatIsVeryLong\n\t\tAND conditionDeltaThatIsVeryLong\n\t\tAND conditionEpsilonThatIsVeryLong\n\t\tAND conditionZetaThatIsVeryLong\n\t)\n\tAND simpleFlag THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
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
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t)\n\tOR (\n\t\tflagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t)\n\tAND flagGateClosed\n\tAND flagInterlockEngaged\n\tAND flagSequenceRunning\n\tAND flagAbortRequested THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "overlong first paren group deep splits second stays whole",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged) OR (flagAbortRequested AND flagResetPermitted) THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t\tAND flagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t\tAND flagGateClosed\n\t\tAND flagInterlockEngaged\n\t)\n\tOR (\n\t\tflagAbortRequested\n\t\tAND flagResetPermitted\n\t) THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "both paren groups overlong deep split at two indent levels",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged) OR (flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete AND flagAutoModeSelected AND flagManualModeSelected) THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t\tAND flagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t\tAND flagGateClosed\n\t\tAND flagInterlockEngaged\n\t)\n\tOR (\n\t\tflagSequenceRunning\n\t\tAND flagAbortRequested\n\t\tAND flagResetPermitted\n\t\tAND flagCycleComplete\n\t\tAND flagAutoModeSelected\n\t\tAND flagManualModeSelected\n\t) THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "nested parens deep split at three indent levels",
			in:   "PROGRAM p\nIF ((flagOperationalReady OR flagMaintenanceOverride) AND (flagEmergencyStopActive OR flagPowerAvailable)) AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\t(\n\t\t\tflagOperationalReady\n\t\t\tOR flagMaintenanceOverride\n\t\t)\n\t\tAND (\n\t\t\tflagEmergencyStopActive\n\t\t\tOR flagPowerAvailable\n\t\t)\n\t)\n\tAND flagGateClosed\n\tAND flagInterlockEngaged\n\tAND flagSequenceRunning\n\tAND flagAbortRequested THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
		},
		{
			name: "mixed top-level paren groups with trailing chain",
			in:   "PROGRAM p\nIF (flagOperationalReady AND flagMaintenanceOverride) OR (flagEmergencyStopActive AND flagPowerAvailable) OR (flagGateClosed AND flagInterlockEngaged) AND flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete THEN\n    x := TRUE;\nEND_IF\nEND_PROGRAM",
			want: "PROGRAM p\nIF (\n\t\tflagOperationalReady\n\t\tAND flagMaintenanceOverride\n\t)\n\tOR (\n\t\tflagEmergencyStopActive\n\t\tAND flagPowerAvailable\n\t)\n\tOR (\n\t\tflagGateClosed\n\t\tAND flagInterlockEngaged\n\t)\n\tAND flagSequenceRunning\n\tAND flagAbortRequested\n\tAND flagResetPermitted\n\tAND flagCycleComplete THEN\n\tx := TRUE;\nEND_IF\nEND_PROGRAM\n",
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

func TestNormalizeComment(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "line comment adds space and capital", in: "//set x", want: "// Set x"},
		{name: "line comment adds space", in: "//set", want: "// Set"},
		{name: "line comment capitalizes", in: "// set x", want: "// Set x"},
		{name: "line comment already correct", in: "// Set x", want: "// Set x"},
		{name: "empty line comment", in: "//", want: "//"},
		{name: "whitespace line comment", in: "// ", want: "//"},
		{name: "block comment adds space and capital", in: "(*block*)", want: "(* Block*)"},
		{name: "block comment capitalizes", in: "(* block *)", want: "(* Block *)"},
		{name: "multiline block comment", in: "(* block\ncomment *)", want: "(* Block\ncomment *)"},
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
