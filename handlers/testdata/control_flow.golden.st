PROGRAM p
VAR
	x : BOOL;
	y : INT;
END_VAR

IF x THEN
	x := TRUE;
ELSIF y > 3 THEN
	y := y + 1;
ELSE
	y := 0;
END_IF

FOR i := 1 TO 10 BY 2 DO
	IF y > 5 THEN
		EXIT;
	END_IF

	i := i + 1;
END_FOR

WHILE x DO
	x := FALSE;
END_WHILE

REPEAT
	y := y - 1;
UNTIL y = 0
END_REPEAT

CASE i OF
	1:
		x := TRUE;

	2, 3:
		y := 5;

	4:
		IF x THEN
			x := TRUE;
		END_IF

	5:
		y := 7;
	ELSE
		y := 0;
END_CASE
END_PROGRAM
