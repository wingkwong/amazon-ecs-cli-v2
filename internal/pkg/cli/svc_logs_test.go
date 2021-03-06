// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type svcLogsMock struct {
	configStore *mocks.Mockstore
	sel         *mocks.MockdeploySelector
}

func TestSvcLogs_Validate(t *testing.T) {
	const (
		mockLimit        = 3
		mockSince        = 1 * time.Minute
		mockStartTime    = "1970-01-01T01:01:01+00:00"
		mockBadStartTime = "badStartTime"
		mockEndTime      = "1971-01-01T01:01:01+00:00"
		mockBadEndTime   = "badEndTime"
	)
	testCases := map[string]struct {
		inputApp       string
		inputSvc       string
		inputLimit     int
		inputFollow    bool
		inputEnvName   string
		inputStartTime string
		inputEndTime   string
		inputSince     time.Duration

		mockstore func(m *mocks.Mockstore)

		wantedError error
	}{
		"with no flag set": {
			// default value for limit and since flags
			inputLimit: 10,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: nil,
		},
		"invalid project name": {
			inputApp: "my-app",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"returns error if since and startTime flags are set together": {
			inputSince:     mockSince,
			inputStartTime: mockStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --since or --start-time may be used"),
		},
		"returns error if follow and endTime flags are set together": {
			inputFollow:  true,
			inputEndTime: mockEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --follow or --end-time may be used"),
		},
		"returns error if invalid start time flag value": {
			inputStartTime: mockBadStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badStartTime for \"--start-time\" flag: reading time value badStartTime: parsing time \"badStartTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badStartTime\" as \"2006\""),
		},
		"returns error if invalid end time flag value": {
			inputEndTime: mockBadEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badEndTime for \"--end-time\" flag: reading time value badEndTime: parsing time \"badEndTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badEndTime\" as \"2006\""),
		},
		"returns error if invalid since flag value": {
			inputSince: -mockSince,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--since must be greater than 0"),
		},
		"returns error if limit value is below limit": {
			inputLimit: -1,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit -1 is out-of-bounds, value must be between 1 and 10000"),
		},
		"returns error if limit value is above limit": {
			inputLimit: 10001,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit 10001 is out-of-bounds, value must be between 1 and 10000"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			tc.mockstore(mockstore)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					follow:         tc.inputFollow,
					limit:          tc.inputLimit,
					envName:        tc.inputEnvName,
					humanStartTime: tc.inputStartTime,
					humanEndTime:   tc.inputEndTime,
					since:          tc.inputSince,
					svcName:        tc.inputSvc,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				configStore:   mockstore,
				initCwLogsSvc: func(*svcLogsOpts, string) error { return nil },
			}

			// WHEN
			err := svcLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcLogs_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp     string
		inputSvc     string
		inputEnvName string

		setupMocks func(mocks svcLogsMock)

		wantedError error
	}{
		"with all flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "mockApp",
						gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: "mockSvc",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"return error if fail to select deployed services": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "mockApp",
						gomock.Any(), gomock.Any()).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
		"with no flag set": {
			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("mockApp", nil),
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "mockApp",
						gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: "mockSvc",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"returns error if fail to select app": {
			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcLogAppNamePrompt, svcLogAppNameHelpPrompt).Return("", errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)

			mocks := svcLogsMock{
				configStore: mockstore,
				sel:         mockSel,
			}

			tc.setupMocks(mocks)

			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					envName: tc.inputEnvName,
					svcName: tc.inputSvc,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				configStore: mockstore,
				sel:         mockSel,
			}

			// WHEN
			err := svcLogs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcLogs_Execute(t *testing.T) {
	mockLastEventTime := map[string]int64{
		"mockLogStreamName": 123456,
	}
	logEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -`,
		},
	}
	moreLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -`,
		},
	}
	logEventsHumanString := `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`
	logEventsJSONString := "{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"GET / HTTP/1.1\\\" 200 -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"FATA some error\\\" - -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"WARN some warning\\\" - -\",\"timestamp\":0}\n"
	testCases := map[string]struct {
		inputApp     string
		inputSvc     string
		inputFollow  bool
		inputEnvName string
		inputJSON    bool

		mockcwlogService func(ctrl *gomock.Controller) map[string]cwlogService

		wantedError   error
		wantedContent string
	}{
		"with no optional flags set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil)

				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError:   nil,
			wantedContent: logEventsHumanString,
		},
		"with json flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",
			inputJSON:    true,

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil)

				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError:   nil,
			wantedContent: logEventsJSONString,
		},
		"with follow flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",
			inputFollow:  true,

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).Return(&cloudwatchlogs.LogEventsOutput{
					Events:        logEvents,
					LastEventTime: mockLastEventTime,
				}, nil)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), mockLastEventTime, gomock.Any()).Return(&cloudwatchlogs.LogEventsOutput{
					Events:        moreLogEvents,
					LastEventTime: nil,
				}, nil)
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError: nil,
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -
`,
		},
		"returns error if fail to get event logs": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			mockcwlogService: func(ctrl *gomock.Controller) map[string]cwlogService {
				m := mocks.NewMockcwlogService(ctrl)
				cwlogServices := make(map[string]cwlogService)
				m.EXPECT().TaskLogEvents(fmt.Sprintf(logGroupNamePattern, "mockApp", "mockEnv", "mockSvc"), make(map[string]int64), gomock.Any()).Return(nil, errors.New("some error"))
				cwlogServices["mockEnv"] = m
				return cwlogServices
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			svcLogs := &svcLogsOpts{
				svcLogsVars: svcLogsVars{
					follow:           tc.inputFollow,
					envName:          tc.inputEnvName,
					svcName:          tc.inputSvc,
					shouldOutputJSON: tc.inputJSON,
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				initCwLogsSvc: func(*svcLogsOpts, string) error { return nil },
				cwlogsSvc:     tc.mockcwlogService(ctrl),
				w:             b,
			}

			// WHEN
			err := svcLogs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}
