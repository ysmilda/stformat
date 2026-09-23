program p
var
hexVal: dword := 16#FF;
binVal: byte := 2#1011;
octVal: byte := 8#77;
mask: dword := 16#FFFFFFFF;
intTyped: dint := DINT#-32768;
wordTyped: word := WORD#16#ABCD;
realTyped: real := REAL#1.5;
boolTyped: bool := BOOL#TRUE;
expVal: real := 1.5e3;
negExp: lreal := 1.5E-3;
dur1: time := T#1H 30M 20S 500MS;
dur2: time := TIME#5S;
dur3: ltime := LTIME#100NS;
todVal: tod := TOD#12:30:45.5;
dateVal: date := DATE#2024-06-15;
dtVal: dt := DT#2024-06-15-12:30:45;
ldtVal: ldt := LDT#2024-06-15-12:30:45.123;
str1: string := 'plain';
str2: wstring := "line1$R$Nline2";
end_var
end_program