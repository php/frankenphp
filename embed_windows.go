//go:build windows

package frankenphp

import (
	"io/fs"

	"golang.org/x/sys/windows"
)

func embeddedAppOwnerID() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}

	return user.User.Sid.String(), nil
}

func embeddedAppPathOwnedByCurrentUser(dir string, _ fs.FileInfo) (bool, error) {
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return false, err
	}

	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return false, err
	}
	if owner.Equals(user.User.Sid) {
		return true, nil
	}

	// Objects created by elevated administrators are owned by the Administrators group
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false, err
	}

	return token.IsElevated() && owner.Equals(admins), nil
}

// restrictEmbeddedAppPath replaces the inherited ACL: C:\Windows\Temp lets every user create entries.
func restrictEmbeddedAppPath(dir string) error {
	sid, err := embeddedAppOwnerID()
	if err != nil {
		return err
	}

	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + sid + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}

	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}

func checkEmbeddedAppParent(string) error {
	return nil
}
