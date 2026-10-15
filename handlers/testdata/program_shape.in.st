// What the formatter does to a whole file rather than to a line.
// Blank lines between POUs, after a declaration block and after a block end are its own decisions.

// A POU that does nothing still comes back with a trailing newline.
PROGRAM Untouched
END_PROGRAM
PROGRAM p
x:=1;
END_PROGRAM

// Two POUs in one file are separated by a blank line.
FUNCTION f
END_FUNCTION
FUNCTION g
END_FUNCTION

// A declaration block is separated from the code that follows it.
PROGRAM decl
VAR
x : BOOL;
END_VAR
x := 1;
END_PROGRAM

// A statement after END_IF starts a new paragraph.
PROGRAM afterEndIf
IF x THEN
    x := TRUE;
END_IF y:=1;
END_PROGRAM

// The same for a block that ends inside another one.
PROGRAM nestedBlockEnd
FOR i:=1 TO 10 DO
    IF x THEN
        EXIT;
    END_IF
    RETURN;
END_FOR
END_PROGRAM

// An empty program is left alone apart from the trailing newline.
PROGRAM P
END_PROGRAM