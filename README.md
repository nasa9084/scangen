scangen
=======

scangen generates *sql.Row scan method for given struct.

## Example:

apply scangen for struct below:

``` go
type SomeStruct struct {
    Name string
    Age  int
}
```

then scangen generates:

``` go
func (someStruct *SomeStruct) Scan(sc interface{
    Scan(...interface{}) error
} error {
    return sc.Scan(&someStruct.Name, &someStruct.Age)
}
```

## KNOWN ISSUE

* when given a filename to arguments, package will be empty
