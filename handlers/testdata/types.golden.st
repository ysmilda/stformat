TYPE point :
	STRUCT
		x : INT;
		y : INT;
	END_STRUCT

	union32 : UNION
		a : INT;
		b : DWORD;
		c : REAL;
	END_UNION

	matrix : ARRAY[1..3, 1..3] OF REAL;
	point_arr : ARRAY[1..4] OF point;
END_TYPE
