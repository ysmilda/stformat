type
point:
struct
x: int;
y: int;
end_struct
union32:
union
a: int;
b: dword;
c: real;
end_union
matrix: array[1..3,1..3] of real;
point_arr: array[1..4] of point;
end_type