// Line breaks in the source are kept where they mean something, and dropped everywhere else.
// A break after an opening bracket, around a comma or before a closing bracket keeps that list
// broken open, one item per line. A break before or after an operator keeps that chain broken.

// The list is broken in the source, so it stays broken although it fits.
PROGRAM keptList
result := Combine(
    first := 1, second := 2);
END_PROGRAM

// A break after a comma is enough.
PROGRAM keptListAtComma
result := Combine(first := 1,
    second := 2);
END_PROGRAM

// The chain is broken in the source, so every operator of it starts a line.
PROGRAM keptChain
total := a +
    b + c;
END_PROGRAM

// The break can also come before the operator.
PROGRAM keptChainBefore
ready := sensorOne
    AND sensorTwo AND sensorThree;
END_PROGRAM

// A broken condition is laid out in full.
PROGRAM keptCondition
IF a
    OR b THEN
    x := 1;
END_IF
END_PROGRAM

// Breaks anywhere else are dropped: after the assignment, between a name and its bracket,
// and inside a list that is not broken at one of its own break points.
PROGRAM dropped
x :=
    5;
y := F(first :=
    1, second := 2);
z := Compute
    (a := 1);
END_PROGRAM

// A broken enum stays broken; one written on a line stays on it.
TYPE E_Broken :
(
    Idle := 0,
    Running := 1
);
END_TYPE

TYPE E_Inline :
(Idle := 0, Running := 1);
END_TYPE
