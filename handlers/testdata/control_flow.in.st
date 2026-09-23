program p
var
x: bool;
y: int;
end_var
if x then
x:=true;
elsif y>3 then
y:=y+1;
else
y:=0;
end_if
for i:=1 to 10 by 2 do
if y>5 then
exit;
end_if
i:=i+1;
end_for
while x do
x:=false;
end_while
repeat
y:=y-1;
until y=0 end_repeat
case i of
1: x:=true;
2,3: y:=5;
4:
if x then
x:=true;
end_if 5: y:=7;
else y:=0;
end_case
end_program