// Code generated by go-swagger; DO NOT EDIT.

package store

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

// GetInventoryReader is a Reader for the GetInventory structure.
type GetInventoryReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *GetInventoryReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewGetInventoryOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, runtime.NewAPIError("response status code does not match any response statuses defined for this endpoint in the swagger spec", response, response.Code())
	}
}

// NewGetInventoryOK creates a GetInventoryOK with default headers values
func NewGetInventoryOK() *GetInventoryOK {
	return &GetInventoryOK{}
}

/* GetInventoryOK describes a response with status code 200, with default header values.

successful operation
*/
type GetInventoryOK struct {
	Payload map[string]int32
}

func (o *GetInventoryOK) Error() string {
	return fmt.Sprintf("[GET /store/inventory][%d] getInventoryOK  %+v", 200, o.Payload)
}
func (o *GetInventoryOK) GetPayload() map[string]int32 {
	return o.Payload
}

func (o *GetInventoryOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
