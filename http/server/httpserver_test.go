package server

import (
	"indigo/tests"
	"indigo/types"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	simpleRequest = []byte("GET / HTTP/1.1\r\nHello: world\r\n\r\n")
	nilRequest    = (*types.Request)(nil)
)

func getPollerOutput(reqChan <-chan *types.Request, errChan <-chan error) (*types.Request, error) {
	select {
	case req := <-reqChan:
		return req, nil
	case err := <-errChan:
		return nil, err
	}
}

func TestHTTPServerRunAbility(t *testing.T) {
	t.Run("RunOnce", func(t *testing.T) {
		mockedParser := tests.HTTPParserMock{
			Actions: []tests.ParserRetVal{
				{true, nil, nil},
			},
		}

		reqChan, errChan := make(requestsChan, 1), make(errorsChan, 2)
		handler := newHTTPHandler(HTTPHandlerArgs{
			Router:     nil,
			Request:    nilRequest,
			Parser:     &mockedParser,
			RespWriter: nil,
		}, reqChan, errChan)

		err := handler.OnData(simpleRequest)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 1, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		req, reqErr := getPollerOutput(reqChan, errChan)
		require.Nil(t, reqErr, "unwanted error")
		require.Equal(t, req, nilRequest, "must be equal")
	})

	t.Run("SplitRequestInto2Parts", func(t *testing.T) {
		firstPart := simpleRequest[:len(simpleRequest)/2]
		secondPart := simpleRequest[len(simpleRequest)/2:]

		mockedParser := tests.HTTPParserMock{
			Actions: []tests.ParserRetVal{
				{false, nil, nil},
				{true, nil, nil},
			},
		}

		reqChan, errChan := make(requestsChan, 1), make(errorsChan, 2)
		handler := newHTTPHandler(HTTPHandlerArgs{
			Router:     nil,
			Request:    nilRequest,
			Parser:     &mockedParser,
			RespWriter: nil,
		}, reqChan, errChan)

		err := handler.OnData(firstPart)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 1, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		err = handler.OnData(secondPart)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 2, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		req, reqErr := getPollerOutput(reqChan, errChan)
		require.Nil(t, reqErr, "unwanted error")
		require.Equal(t, req, nilRequest, "must be equal")
	})

}

func TestHTTPServer2Requests(t *testing.T) {
	t.Run("2Requests", func(t *testing.T) {
		mockedParser := tests.HTTPParserMock{
			Actions: []tests.ParserRetVal{
				{true, nil, nil},
				{true, nil, nil},
			},
		}

		reqChan, errChan := make(requestsChan, 1), make(errorsChan, 2)
		handler := newHTTPHandler(HTTPHandlerArgs{
			Router:     nil,
			Request:    nilRequest,
			Parser:     &mockedParser,
			RespWriter: nil,
		}, reqChan, errChan)

		err := handler.OnData(simpleRequest)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 1, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		req, reqErr := getPollerOutput(reqChan, errChan)
		require.Nil(t, reqErr, "unwanted error")
		require.Equal(t, req, nilRequest, "must be equal")

		err = handler.OnData(simpleRequest)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 2, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		req, reqErr = getPollerOutput(reqChan, errChan)
		require.Nil(t, reqErr, "unwanted error")
		require.Equal(t, req, nilRequest, "must be equal")
	})

	t.Run("2RequestsWithExtra", func(t *testing.T) {
		// just copy to be sure no implicit shit will happen
		request := append(make([]byte, 0, len(simpleRequest)), simpleRequest...)
		firstRequest := append(request, simpleRequest[:len(simpleRequest)/2]...)
		secondRequest := simpleRequest[len(simpleRequest)/2:]

		mockedParser := tests.HTTPParserMock{
			Actions: []tests.ParserRetVal{
				{true, nil, nil},
				{true, nil, nil},
			},
		}

		reqChan, errChan := make(requestsChan, 1), make(errorsChan, 2)
		handler := newHTTPHandler(HTTPHandlerArgs{
			Router:     nil,
			Request:    nilRequest,
			Parser:     &mockedParser,
			RespWriter: nil,
		}, reqChan, errChan)

		err := handler.OnData(firstRequest)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 1, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		req, reqErr := getPollerOutput(reqChan, errChan)
		require.Nil(t, reqErr, "unwanted error")
		require.Equal(t, req, nilRequest, "must be equal")

		err = handler.OnData(secondRequest)
		require.Nil(t, err, "unwanted error")
		require.Equal(t, 2, mockedParser.CallsCount(), "too much parser calls")
		require.Nil(t, mockedParser.GetError(), "unwanted error")

		req, reqErr = getPollerOutput(reqChan, errChan)
		require.Nil(t, reqErr, "unwanted error")
		require.Equal(t, req, nilRequest, "must be equal")
	})
}
