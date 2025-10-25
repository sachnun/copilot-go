package approval

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	appErr "internal/errors"
)

func AwaitApproval() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Accept incoming request? [y/N]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "y" || input == "yes" {
		return nil
	}

	resp := appErr.NewJSONResponse(403, map[string]any{
		"message": "Request rejected",
	})
	return appErr.NewHTTPError("Request rejected", resp)
}
