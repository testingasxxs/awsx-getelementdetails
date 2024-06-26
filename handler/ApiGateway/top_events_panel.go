package ApiGateway

import (
	"fmt"
	"log"
	"time"

	"github.com/Appkube-awsx/awsx-common/cmdb"
	"github.com/Appkube-awsx/awsx-common/config"

	"github.com/Appkube-awsx/awsx-common/authenticate"
	"github.com/Appkube-awsx/awsx-common/awsclient"
	"github.com/Appkube-awsx/awsx-common/model"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/spf13/cobra"
)

var AwsxApiTopEventCmd = &cobra.Command{

	Use:   "top_events_panel",
	Short: "Get top event metrics data",
	Long:  `Command to get top event metrics data`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running top event panel command")

		var authFlag bool
		var clientAuth *model.Auth
		var err error
		authFlag, clientAuth, err = authenticate.AuthenticateCommand(cmd)

		if err != nil {
			log.Printf("Error during authentication: %v\n", err)
			err := cmd.Help()
			if err != nil {
				return
			}
			return
		}
		if authFlag {
			panel, err := GetTopEventsData(cmd, clientAuth, nil)
			if err != nil {
				return
			}
			fmt.Println(panel)
		}
	},
}

func GetTopEventsData(cmd *cobra.Command, clientAuth *model.Auth, cloudWatchLogs *cloudwatchlogs.CloudWatchLogs) ([]*cloudwatchlogs.GetQueryResultsOutput, error) {
	elementId, _ := cmd.PersistentFlags().GetString("elementId")
	cmdbApiUrl, _ := cmd.PersistentFlags().GetString("cmdbApiUrl")
	logGroupName, _ := cmd.PersistentFlags().GetString("logGroupName")

	if elementId != "" {
		log.Println("getting cloud-element data from cmdb")
		apiUrl := cmdbApiUrl
		if cmdbApiUrl == "" {
			log.Println("using default cmdb url")
			apiUrl = config.CmdbUrl
		}
		log.Println("cmdb url: " + apiUrl)
		cmdbData, err := cmdb.GetCloudElementData(apiUrl, elementId)
		if err != nil {
			return nil, err
		}
		logGroupName = cmdbData.LogGroup
	}

	startTimeStr, _ := cmd.PersistentFlags().GetString("startTime")
	endTimeStr, _ := cmd.PersistentFlags().GetString("endTime")
	var startTime, endTime *time.Time

	// Parse start time if provided
	if startTimeStr != "" {
		parsedStartTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			log.Printf("Error parsing start time: %v", err)
			err := cmd.Help()
			if err != nil {
				// handle error
			}
		}
		startTime = &parsedStartTime
	} else {
		defaultStartTime := time.Now().Add(-5 * time.Minute)
		startTime = &defaultStartTime
	}

	if endTimeStr != "" {
		parsedEndTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			log.Printf("Error parsing end time: %v", err)
			err := cmd.Help()
			if err != nil {
				// handle error
			}
		}
		endTime = &parsedEndTime
	} else {
		defaultEndTime := time.Now()
		endTime = &defaultEndTime
	}

	results, err := FilterCloudWatchLog(clientAuth, startTime, endTime, logGroupName, cloudWatchLogs)
	if err != nil {
		return nil, nil
	}
	processedResults := ProcessQueryResults(results)

	return processedResults, nil

}

func FilterCloudWatchLog(clientAuth *model.Auth, startTime, endTime *time.Time, logGroupName string, cloudWatchLogs *cloudwatchlogs.CloudWatchLogs) ([]*cloudwatchlogs.GetQueryResultsOutput, error) {
	params := &cloudwatchlogs.StartQueryInput{
		LogGroupName: aws.String(logGroupName),
		StartTime:    aws.Int64(startTime.Unix() * 1000),
		EndTime:      aws.Int64(endTime.Unix() * 1000),
		QueryString: aws.String(`fields @timestamp, eventName, @message
		| filter eventSource = 'apigateway.amazonaws.com'
		| stats count() as count by eventName, @timestamp
		| limit 60
		`),
	}

	if cloudWatchLogs == nil {
		cloudWatchLogs = awsclient.GetClient(*clientAuth, awsclient.CLOUDWATCH_LOG).(*cloudwatchlogs.CloudWatchLogs)
	}

	queryResult, err := cloudWatchLogs.StartQuery(params)
	if err != nil {
		return nil, fmt.Errorf("failed to start query: %v", err)

	}
	queryId := queryResult.QueryId
	var queryResults []*cloudwatchlogs.GetQueryResultsOutput

	for {
		// Check query status
		queryStatusInput := &cloudwatchlogs.GetQueryResultsInput{
			QueryId: queryId,
		}

		queryResult, err := cloudWatchLogs.GetQueryResults(queryStatusInput)
		if err != nil {
			return nil, fmt.Errorf("failed to get query results: %v", err)
		}

		queryResults = append(queryResults, queryResult)

		if *queryResult.Status != "Complete" {
			time.Sleep(5 * time.Second) // wait before querying again
			continue
		}

		break // exit loop if query is complete
	}
	return queryResults, nil
}

func ProcessQueryResults(results []*cloudwatchlogs.GetQueryResultsOutput) []*cloudwatchlogs.GetQueryResultsOutput {
	processedResults := make([]*cloudwatchlogs.GetQueryResultsOutput, 0)

	for _, result := range results {
		if *result.Status == "Complete" {
			for _, resultField := range result.Results {
				for _, data := range resultField {
					if *data.Field == "eventName" {

						log.Printf("eventName: %s\n", *data)

					}
				}
			}
			processedResults = append(processedResults, result)
		} else {
			log.Println("Query status is not complete.")
		}
	}

	return processedResults
}

func init() {
	AwsxApiTopEventCmd.PersistentFlags().String("logGroupName", "", "log group name")
	AwsxApiTopEventCmd.PersistentFlags().String("functionName", "", "Lambda function name")
	AwsxApiTopEventCmd.PersistentFlags().String("startTime", "", "start time")
	AwsxApiTopEventCmd.PersistentFlags().String("endTime", "", "end time")
}
