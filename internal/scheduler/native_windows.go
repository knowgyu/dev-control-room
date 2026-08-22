//go:build windows

package scheduler

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	clsctxInprocServer = 1
	coinApartment      = 2

	dispatchMethod              = 1
	dispatchPropertyGet         = 2
	dispatchPropertyPut         = 4
	dispatchPropertyPutID int32 = -3

	vtEmpty    = 0
	vtI4       = 3
	vtBSTR     = 8
	vtDispatch = 9
	vtBool     = 11

	taskCreateOrUpdate        = 6
	taskLogonInteractiveToken = 3
	taskTriggerDaily          = 2
	taskTriggerLogon          = 9
	taskActionExec            = 0
	taskInstancesIgnoreNew    = 2
)

var (
	ole32            = syscall.NewLazyDLL("ole32.dll")
	oleaut32         = syscall.NewLazyDLL("oleaut32.dll")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	coUninitialize   = ole32.NewProc("CoUninitialize")
	sysAllocString   = oleaut32.NewProc("SysAllocString")
	sysStringLen     = oleaut32.NewProc("SysStringLen")
	variantClear     = oleaut32.NewProc("VariantClear")

	clsidTaskScheduler = guid{0x0f87369f, 0xa4e5, 0x4cfc, [8]byte{0xbd, 0x3e, 0x73, 0xe6, 0x15, 0x45, 0x72, 0xdd}}
	iidTaskService     = guid{0x2faba4c7, 0x4da9, 0x4013, [8]byte{0x96, 0x97, 0x20, 0xcc, 0x3f, 0xd4, 0x0f, 0x85}}
	iidTaskFolder      = guid{0x8cfac062, 0xa080, 0x4c15, [8]byte{0x9a, 0x88, 0xaa, 0x7c, 0x2a, 0xf8, 0x0d, 0xfe}}
	iidTaskDefinition  = guid{0xf5bc8fc5, 0x536d, 0x4f77, [8]byte{0xb8, 0x52, 0xfb, 0xc1, 0x35, 0x6f, 0xde, 0xb6}}
	iidNull            guid
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// VARIANT is deliberately fixed-width on both 32-bit and 64-bit Windows.
type variant struct {
	VT        uint16
	Reserved1 uint16
	Reserved2 uint16
	Reserved3 uint16
	Value     int64
}

type dispParams struct {
	Args          *variant
	NamedArgs     *int32
	ArgCount      uint32
	NamedArgCount uint32
}

type dispatch struct{ ptr unsafe.Pointer }

type hresult uintptr

func (e hresult) Error() string { return fmt.Sprintf("Task Scheduler COM error 0x%08X", uint32(e)) }

// NativeAdapter mutates only the fixed application-owned task through the
// Task Scheduler COM API. It never invokes a shell or PowerShell.
type NativeAdapter struct{}

func (a *NativeAdapter) Apply(ctx context.Context, operation Operation) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := Validate(operation); err != nil {
		return Result{}, err
	}
	if operation.Kind == OperationDryRun {
		return Result{Operation: operation, Message: "validated native Task Scheduler definition; no task was changed"}, nil
	}
	if err := comInitialize(); err != nil {
		return Result{}, err
	}
	defer coUninitialize.Call()

	service, err := newTaskService()
	if err != nil {
		return Result{}, err
	}
	defer service.release()
	if err := service.connect(); err != nil {
		return Result{}, err
	}
	root, err := service.rootFolder()
	if err != nil {
		return Result{}, err
	}
	defer root.release()

	switch operation.Kind {
	case OperationInstall:
		if err := install(service, root, operation); err != nil {
			return Result{}, err
		}
		return Result{Operation: operation, Applied: true, Exists: true, Message: "installed native Windows Task Scheduler task"}, nil
	case OperationUninstall:
		exists, err := taskExists(root)
		if err != nil {
			return Result{}, err
		}
		if exists {
			if err := root.call("DeleteTask", dispatchMethod, bstrVariant(AppTaskName), i4Variant(0)); err != nil {
				return Result{}, err
			}
		}
		return Result{Operation: operation, Applied: exists, Exists: false, Message: "removed native Windows Task Scheduler task"}, nil
	case OperationStatus:
		exists, err := status(root, operation)
		if err != nil {
			return Result{}, err
		}
		return Result{Operation: operation, Exists: exists, Message: "native Windows Task Scheduler status"}, nil
	default:
		return Result{}, fmt.Errorf("unsupported scheduler operation")
	}
}

func install(service, root *dispatch, operation Operation) error {
	exists, err := taskExists(root)
	if err != nil {
		return err
	}
	if exists {
		if _, err := status(root, operation); err != nil {
			return fmt.Errorf("refusing to replace incompatible application task: %w", err)
		}
	}
	definition, err := service.newTask()
	if err != nil {
		return err
	}
	defer definition.release()
	settings, err := definition.propertyDispatch("Settings")
	if err != nil {
		return err
	}
	defer settings.release()
	if err := settings.put("StartWhenAvailable", boolVariant(true)); err != nil {
		return err
	}
	if err := settings.put("MultipleInstances", i4Variant(taskInstancesIgnoreNew)); err != nil {
		return err
	}
	// A task running at logon must not be stopped merely because a daily
	// trigger also fires; the default unlimited execution time is retained.
	if err := configureTriggers(definition); err != nil {
		return err
	}
	if err := configureAction(definition, operation); err != nil {
		return err
	}
	return registerTaskDefinition(root, definition)
}

func registerTaskDefinition(root, definition *dispatch) error {
	folder, err := root.query(iidTaskFolder)
	if err != nil {
		return fmt.Errorf("query ITaskFolder: %w", err)
	}
	defer folder.release()
	taskDefinition, err := definition.query(iidTaskDefinition)
	if err != nil {
		return fmt.Errorf("query ITaskDefinition: %w", err)
	}
	defer taskDefinition.release()
	path, err := syscall.UTF16PtrFromString(AppTaskName)
	if err != nil {
		return err
	}
	user, password, sddl := emptyVariant(), emptyVariant(), emptyVariant()
	var registered unsafe.Pointer
	vtable := *(**[17]uintptr)(folder.ptr)
	result, _, _ := syscall.SyscallN(vtable[16], uintptr(folder.ptr), uintptr(unsafe.Pointer(path)), uintptr(taskDefinition.ptr), taskCreateOrUpdate, uintptr(unsafe.Pointer(&user)), uintptr(unsafe.Pointer(&password)), taskLogonInteractiveToken, uintptr(unsafe.Pointer(&sddl)), uintptr(unsafe.Pointer(&registered)))
	if result != 0 {
		return hresult(result)
	}
	(&dispatch{ptr: registered}).release()
	return nil
}

func configureTriggers(definition *dispatch) error {
	triggers, err := definition.propertyDispatch("Triggers")
	if err != nil {
		return err
	}
	defer triggers.release()
	logon, err := triggers.getDispatch("Create", i4Variant(taskTriggerLogon))
	if err != nil {
		return err
	}
	defer logon.release()
	if err := logon.put("Enabled", boolVariant(true)); err != nil {
		return err
	}
	daily, err := triggers.getDispatch("Create", i4Variant(taskTriggerDaily))
	if err != nil {
		return err
	}
	defer daily.release()
	if err := daily.put("Enabled", boolVariant(true)); err != nil {
		return err
	}
	if err := daily.put("StartBoundary", bstrVariant(DailyStartBoundary)); err != nil {
		return err
	}
	return daily.put("DaysInterval", i4Variant(1))
}

func configureAction(definition *dispatch, operation Operation) error {
	actions, err := definition.propertyDispatch("Actions")
	if err != nil {
		return err
	}
	defer actions.release()
	action, err := actions.getDispatch("Create", i4Variant(taskActionExec))
	if err != nil {
		return err
	}
	defer action.release()
	if err := action.put("Path", bstrVariant(operation.ExecutablePath)); err != nil {
		return err
	}
	if err := action.put("Arguments", bstrVariant(windowsCommandLine(operation.Arguments))); err != nil {
		return err
	}
	return action.put("WorkingDirectory", bstrVariant(operation.Arguments[2]))
}

func status(root *dispatch, operation Operation) (bool, error) {
	task, err := root.getDispatch("GetTask", bstrVariant(AppTaskName))
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	defer task.release()
	definition, err := task.propertyDispatch("Definition")
	if err != nil {
		return false, err
	}
	defer definition.release()
	if err := verifySettings(definition); err != nil {
		return false, err
	}
	if err := verifyAction(definition, operation); err != nil {
		return false, err
	}
	if err := verifyTriggers(definition); err != nil {
		return false, err
	}
	return true, nil
}

func taskExists(root *dispatch) (bool, error) {
	task, err := root.getDispatch("GetTask", bstrVariant(AppTaskName))
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	task.release()
	return true, nil
}

func verifySettings(definition *dispatch) error {
	settings, err := definition.propertyDispatch("Settings")
	if err != nil {
		return err
	}
	defer settings.release()
	catchUp, err := settings.propertyBool("StartWhenAvailable")
	if err != nil {
		return err
	}
	multiple, err := settings.propertyI4("MultipleInstances")
	if err != nil {
		return err
	}
	if !catchUp || multiple != taskInstancesIgnoreNew {
		return fmt.Errorf("registered scheduler task does not match duplicate-instance or catch-up policy")
	}
	return nil
}

func verifyAction(definition *dispatch, operation Operation) error {
	actions, err := definition.propertyDispatch("Actions")
	if err != nil {
		return err
	}
	defer actions.release()
	count, err := actions.propertyI4("Count")
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("registered scheduler task has unexpected action count")
	}
	action, err := actions.getDispatch("Item", i4Variant(1))
	if err != nil {
		return err
	}
	defer action.release()
	path, err := action.propertyString("Path")
	if err != nil {
		return err
	}
	arguments, err := action.propertyString("Arguments")
	if err != nil {
		return err
	}
	workingDirectory, err := action.propertyString("WorkingDirectory")
	if err != nil {
		return err
	}
	if path != operation.ExecutablePath || arguments != windowsCommandLine(operation.Arguments) || workingDirectory != operation.Arguments[2] {
		return fmt.Errorf("registered scheduler task does not match typed serve command")
	}
	return nil
}

func verifyTriggers(definition *dispatch) error {
	triggers, err := definition.propertyDispatch("Triggers")
	if err != nil {
		return err
	}
	defer triggers.release()
	count, err := triggers.propertyI4("Count")
	if err != nil {
		return err
	}
	if count != 2 {
		return fmt.Errorf("registered scheduler task has unexpected trigger count")
	}
	seenLogon, seenDaily := false, false
	for i := int32(1); i <= count; i++ {
		trigger, err := triggers.getDispatch("Item", i4Variant(i))
		if err != nil {
			return err
		}
		kind, kindErr := trigger.propertyI4("Type")
		enabled, enabledErr := trigger.propertyBool("Enabled")
		if kindErr != nil || enabledErr != nil {
			trigger.release()
			if kindErr != nil {
				return kindErr
			}
			return enabledErr
		}
		switch kind {
		case taskTriggerLogon:
			seenLogon = enabled
		case taskTriggerDaily:
			boundary, boundaryErr := trigger.propertyString("StartBoundary")
			days, daysErr := trigger.propertyI4("DaysInterval")
			if boundaryErr != nil || daysErr != nil {
				trigger.release()
				if boundaryErr != nil {
					return boundaryErr
				}
				return daysErr
			}
			seenDaily = enabled && boundary == DailyStartBoundary && days == 1
		}
		trigger.release()
	}
	if !seenLogon || !seenDaily {
		return fmt.Errorf("registered scheduler task does not match logon and daily catch-up triggers")
	}
	return nil
}

func comInitialize() error {
	result, _, _ := syscall.SyscallN(syscall.NewLazyDLL("ole32.dll").NewProc("CoInitializeEx").Addr(), 0, coinApartment)
	if result != 0 && result != 1 {
		return hresult(result)
	}
	return nil
}

func newTaskService() (*dispatch, error) {
	var ptr unsafe.Pointer
	result, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidTaskScheduler)), 0, clsctxInprocServer, uintptr(unsafe.Pointer(&iidTaskService)), uintptr(unsafe.Pointer(&ptr)))
	if result != 0 {
		return nil, hresult(result)
	}
	return &dispatch{ptr: ptr}, nil
}

func (d *dispatch) connect() error {
	server, user, domain, password := emptyVariant(), emptyVariant(), emptyVariant(), emptyVariant()
	vtable := *(**[13]uintptr)(d.ptr)
	result, _, _ := syscall.SyscallN(vtable[12], uintptr(d.ptr), uintptr(unsafe.Pointer(&server)), uintptr(unsafe.Pointer(&user)), uintptr(unsafe.Pointer(&domain)), uintptr(unsafe.Pointer(&password)))
	if result != 0 {
		return hresult(result)
	}
	return nil
}

func (d *dispatch) rootFolder() (*dispatch, error) {
	path, err := syscall.UTF16PtrFromString(`\`)
	if err != nil {
		return nil, err
	}
	var folder unsafe.Pointer
	vtable := *(**[14]uintptr)(d.ptr)
	result, _, _ := syscall.SyscallN(vtable[13], uintptr(d.ptr), uintptr(unsafe.Pointer(path)), uintptr(unsafe.Pointer(&folder)))
	if result != 0 {
		return nil, hresult(result)
	}
	return &dispatch{ptr: folder}, nil
}

func (d *dispatch) newTask() (*dispatch, error) {
	var definition unsafe.Pointer
	vtable := *(**[16]uintptr)(d.ptr)
	result, _, _ := syscall.SyscallN(vtable[15], uintptr(d.ptr), 0, uintptr(unsafe.Pointer(&definition)))
	if result != 0 {
		return nil, hresult(result)
	}
	return &dispatch{ptr: definition}, nil
}

func (d *dispatch) release() {
	if d == nil || d.ptr == nil {
		return
	}
	vtable := *(**[3]uintptr)(d.ptr)
	syscall.SyscallN(vtable[2], uintptr(d.ptr))
	d.ptr = nil
}

func (d *dispatch) query(iid guid) (*dispatch, error) {
	if d == nil || d.ptr == nil {
		return nil, fmt.Errorf("Task Scheduler automation object is closed")
	}
	vtable := *(**[3]uintptr)(d.ptr)
	var resultPointer unsafe.Pointer
	result, _, _ := syscall.SyscallN(vtable[0], uintptr(d.ptr), uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&resultPointer)))
	if result != 0 {
		return nil, hresult(result)
	}
	return &dispatch{ptr: resultPointer}, nil
}

func (d *dispatch) call(name string, flags uint16, args ...variant) error {
	_, err := d.invoke(name, flags, args...)
	return err
}

func (d *dispatch) getDispatch(name string, args ...variant) (*dispatch, error) {
	value, err := d.invoke(name, dispatchMethod, args...)
	if err != nil {
		return nil, err
	}
	defer clearVariant(&value)
	if value.VT != vtDispatch || value.Value == 0 {
		return nil, fmt.Errorf("Task Scheduler %s did not return an automation object", name)
	}
	ptr := unsafe.Pointer(uintptr(value.Value))
	value.VT, value.Value = vtEmpty, 0
	return &dispatch{ptr: ptr}, nil
}

func (d *dispatch) propertyDispatch(name string) (*dispatch, error) {
	value, err := d.invoke(name, dispatchPropertyGet)
	if err != nil {
		return nil, err
	}
	defer clearVariant(&value)
	if value.VT != vtDispatch || value.Value == 0 {
		return nil, fmt.Errorf("Task Scheduler property %s was not an automation object", name)
	}
	ptr := unsafe.Pointer(uintptr(value.Value))
	value.VT, value.Value = vtEmpty, 0
	return &dispatch{ptr: ptr}, nil
}

func (d *dispatch) propertyBool(name string) (bool, error) {
	value, err := d.invoke(name, dispatchPropertyGet)
	if err != nil {
		return false, err
	}
	defer clearVariant(&value)
	if value.VT != vtBool {
		return false, fmt.Errorf("Task Scheduler property %s was not boolean", name)
	}
	return int16(value.Value) != 0, nil
}

func (d *dispatch) propertyI4(name string) (int32, error) {
	value, err := d.invoke(name, dispatchPropertyGet)
	if err != nil {
		return 0, err
	}
	defer clearVariant(&value)
	if value.VT != vtI4 {
		return 0, fmt.Errorf("Task Scheduler property %s was not integer", name)
	}
	return int32(value.Value), nil
}

func (d *dispatch) propertyString(name string) (string, error) {
	value, err := d.invoke(name, dispatchPropertyGet)
	if err != nil {
		return "", err
	}
	defer clearVariant(&value)
	if value.VT != vtBSTR {
		return "", fmt.Errorf("Task Scheduler property %s was not text", name)
	}
	return variantString(value), nil
}

func (d *dispatch) put(name string, value variant) error {
	_, err := d.invoke(name, dispatchPropertyPut, value)
	return err
}

func (d *dispatch) invoke(name string, flags uint16, args ...variant) (variant, error) {
	if d == nil || d.ptr == nil {
		return variant{}, fmt.Errorf("Task Scheduler automation object is closed")
	}
	nameUTF16, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return variant{}, err
	}
	vtable := *(**[7]uintptr)(d.ptr)
	var dispid int32
	result, _, _ := syscall.SyscallN(vtable[5], uintptr(d.ptr), uintptr(unsafe.Pointer(&iidNull)), uintptr(unsafe.Pointer(&nameUTF16)), 1, 0x0400, uintptr(unsafe.Pointer(&dispid)))
	if result != 0 {
		return variant{}, fmt.Errorf("Task Scheduler GetIDsOfNames %s: %w", name, hresult(result))
	}
	// IDispatch consumes positional arguments in reverse order.
	for left, right := 0, len(args)-1; left < right; left, right = left+1, right-1 {
		args[left], args[right] = args[right], args[left]
	}
	var params dispParams
	if len(args) != 0 {
		params.Args, params.ArgCount = &args[0], uint32(len(args))
	}
	var named int32
	if flags == dispatchPropertyPut {
		named = dispatchPropertyPutID
		params.NamedArgs, params.NamedArgCount = &named, 1
	}
	var output variant
	var argumentError uint32
	result, _, _ = syscall.SyscallN(vtable[6], uintptr(d.ptr), uintptr(dispid), uintptr(unsafe.Pointer(&iidNull)), 0x0400, uintptr(flags), uintptr(unsafe.Pointer(&params)), uintptr(unsafe.Pointer(&output)), 0, uintptr(unsafe.Pointer(&argumentError)))
	for i := range args {
		clearVariant(&args[i])
	}
	if result != 0 {
		return variant{}, fmt.Errorf("Task Scheduler Invoke %s argument %d: %w", name, argumentError, hresult(result))
	}
	return output, nil
}

func emptyVariant() variant         { return variant{VT: vtEmpty} }
func i4Variant(value int32) variant { return variant{VT: vtI4, Value: int64(value)} }
func boolVariant(value bool) variant {
	if value {
		return variant{VT: vtBool, Value: -1}
	}
	return variant{VT: vtBool}
}
func bstrVariant(value string) variant {
	text, _ := syscall.UTF16PtrFromString(value)
	pointer, _, _ := sysAllocString.Call(uintptr(unsafe.Pointer(text)))
	return variant{VT: vtBSTR, Value: int64(pointer)}
}
func clearVariant(value *variant) {
	if value == nil || value.VT == vtEmpty {
		return
	}
	variantClear.Call(uintptr(unsafe.Pointer(value)))
	value.VT, value.Value = vtEmpty, 0
}
func variantString(value variant) string {
	if value.Value == 0 {
		return ""
	}
	length, _, _ := sysStringLen.Call(uintptr(value.Value))
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(uintptr(value.Value))), int(length)))
}
func isNotFound(err error) bool { value, ok := err.(hresult); return ok && uint32(value) == 0x80070002 }

func windowsCommandLine(args []string) string {
	result := ""
	for index, argument := range args {
		if index != 0 {
			result += " "
		}
		result += quoteWindowsArgument(argument)
	}
	return result
}

func quoteWindowsArgument(argument string) string {
	if argument != "" && !containsWindowsArgumentWhitespace(argument) {
		return argument
	}
	result := `"`
	backslashes := 0
	for _, runeValue := range argument {
		if runeValue == '\\' {
			backslashes++
			continue
		}
		if runeValue == '"' {
			result += repeatBackslash(backslashes*2+1) + `"`
			backslashes = 0
			continue
		}
		result += repeatBackslash(backslashes) + string(runeValue)
		backslashes = 0
	}
	return result + repeatBackslash(backslashes*2) + `"`
}
func containsWindowsArgumentWhitespace(value string) bool {
	for _, r := range value {
		if r == ' ' || r == '\t' {
			return true
		}
	}
	return false
}
func repeatBackslash(count int) string {
	result := ""
	for ; count > 0; count-- {
		result += `\`
	}
	return result
}
