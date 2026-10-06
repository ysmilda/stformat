PROGRAM Sections
VAR
	a : INT;
END_VAR

// stformat:off
VAR
	ugly    :   INT;
END_VAR
// stformat:on

a := 1; // stformat:off
   b   :=   2;
// stformat:on

(* stformat:off *)
	untouched   :=   1;
(* stformat:on *)

// STFORMAT: OFF
   another    :=   2;
// stformat : on

// stformat:ignore is only a directive in the leading comment block, so here it
// is an ordinary comment.
b := 4; // stformat:ignore
c := 5;

IF a > 0 THEN
	aLongFunctionName(firstArgument := aValueWithALongName, secondArgument := anotherValueWithALongName, thirdArgument := yetAnotherValue); // an explanation of the call
END_IF
END_PROGRAM