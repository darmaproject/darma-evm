(module
 (type $0 (func (param i32) (result i32)))
 (type $1 (func (param i32 i64) (result i64)))
 (type $2 (func (param i64)))
 (global $global$0 i32 (i32.const 32))
 (table 0 0 anyfunc)
 (memory $0 1 1)
 (export "first_api" (func $1))
 (export "second_api" (func $2))
 (export "malloc" (func $0))
 (export "default" (memory $0))
 (func $0 (; 0 ;) (type $0) (param $var$0 i32) (result i32)
  (local $var$1 i32)
  (set_global $global$0
   (i32.add
    (tee_local $var$1
     (get_global $global$0)
    )
    (get_local $var$0)
   )
  )
  (get_local $var$1)
 )
 (func $1 (; 1 ;) (type $1) (param $var$0 i32) (param $var$1 i64) (result i64)
  (i64.add
   (i64.extend_s/i32
    (get_local $var$0)
   )
   (get_local $var$1)
  )
 )
 (func $2 (; 2 ;) (type $2) (param $var$0 i64)
  (nop)
 )
)
