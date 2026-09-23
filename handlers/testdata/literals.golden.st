PROGRAM p
VAR
	hexVal    : DWORD := 16#FF;
	binVal    : BYTE := 2#1011;
	octVal    : BYTE := 8#77;
	mask      : DWORD := 16#FFFFFFFF;
	intTyped  : DINT := DINT#-32768;
	wordTyped : WORD := WORD#16#ABCD;
	realTyped : REAL := REAL#1.5;
	boolTyped : BOOL := BOOL#TRUE;
	expVal    : REAL := 1.5e3;
	negExp    : LREAL := 1.5E-3;
	dur1      : TIME := T#1H 30M 20S 500MS;
	dur2      : TIME := TIME#5S;
	dur3      : LTIME := LTIME#100NS;
	todVal    : TOD := TOD#12:30:45.5;
	dateVal   : DATE := DATE#2024-06-15;
	dtVal     : DT := DT#2024-06-15-12:30:45;
	ldtVal    : LDT := LDT#2024-06-15-12:30:45.123;
	str1      : STRING := 'plain';
	str2      : WSTRING := "line1$R$Nline2";
END_VAR
END_PROGRAM
