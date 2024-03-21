package EC2

import (
	"encoding/json"
	"fmt"

	"github.com/Appkube-awsx/awsx-common/authenticate"
	"github.com/Appkube-awsx/awsx-common/awsclient"
	"github.com/Appkube-awsx/awsx-common/cmdb"
	"github.com/Appkube-awsx/awsx-common/config"
	"github.com/Appkube-awsx/awsx-common/model"
	"github.com/aws/aws-sdk-go/aws"

	"log"
	"time"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/spf13/cobra"
)

type CpuUtilizationResult struct {
	AverageUsage float64 `json:"averageUsage"`
}

var AwsxEc2CpuUtilizationgraphCmd = &cobra.Command{
	Use:   "cpu_utilization_graph_panel",
	Short: "get cpu utilization graph metrics data",
	Long:  `command to get cpu utilization graph metrics data`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("running from child command")
		var authFlag, clientAuth, err = authenticate.AuthenticateCommand(cmd)
		if err != nil {
			log.Printf("Error during authentication: %v\n", err)
			err := cmd.Help()
			if err != nil {
				return
			}
			return
		}
		if authFlag {
			responseType, _ := cmd.PersistentFlags().GetString("responseType")
			jsonResp, cloudwatchMetricResp, err := GetCpuUtilizationGraphPanel(cmd, clientAuth, nil)
			if err != nil {
				log.Println("Error getting cpu utilization graph: ", err)
				return
			}
			if responseType == "frame" {
				fmt.Println(cloudwatchMetricResp)
			} else {
				// default case. it prints json
				fmt.Println(jsonResp)
			}
		}

	},
}

func GetCpuUtilizationGraphPanel(cmd *cobra.Command, clientAuth *model.Auth, cloudWatchClient *cloudwatch.CloudWatch) (string, map[string]*cloudwatch.GetMetricDataOutput, error) {
	elementId, _ := cmd.PersistentFlags().GetString("elementId")
	elementType, _ := cmd.PersistentFlags().GetString("elementType")
	cmdbApiUrl, _ := cmd.PersistentFlags().GetString("cmdbApiUrl")
	instanceId, _ := cmd.PersistentFlags().GetString("instanceId")

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
			return "", nil, err
		}
		instanceId = cmdbData.InstanceId

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
				return "", nil, err
			}
			return "", nil, err
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
				return "", nil, err
			}
			return "", nil, err
		}
		endTime = &parsedEndTime
	} else {
		defaultEndTime := time.Now()
		endTime = &defaultEndTime
	}
	cloudwatchMetricData := map[string]*cloudwatch.GetMetricDataOutput{}
	
	// Get average usage
	rawData, err := GetCpuUtilizationGraphMetricData(clientAuth, instanceId, elementType, startTime, endTime, "Average", cloudWatchClient)
	if err != nil {
		log.Println("Error in getting rawdata: ", err)
		return "", nil, err
	}
	cloudwatchMetricData["AverageUsage"] = rawData
	
	jsonOutput := CpuUtilizationResult{
		AverageUsage: *rawData.MetricDataResults[0].Values[0],
	}

	jsonString, err := json.Marshal(jsonOutput)
	if err != nil {
		log.Println("Error in marshalling json in string: ", err)
		return "", nil, err
	}

	return string(jsonString), cloudwatchMetricData, nil

}

func GetCpuUtilizationGraphMetricData(clientAuth *model.Auth, instanceID, elementType string, startTime, endTime *time.Time, statistic string, cloudWatchClient *cloudwatch.CloudWatch) (*cloudwatch.GetMetricDataOutput, error) {
	log.Printf("Getting metric data for instance %s in namespace %s from %v to %v", instanceID, elementType, startTime, endTime)

	elmType := "AWS/EC2"
	if elementType == "EC2" {
		elmType = "AWS/" + elementType
	}
	input := &cloudwatch.GetMetricDataInput{
		EndTime:   endTime,
		StartTime: startTime,
		MetricDataQueries: []*cloudwatch.MetricDataQuery{
			{
				Id: aws.String("m1"),
				MetricStat: &cloudwatch.MetricStat{
					Metric: &cloudwatch.Metric{
						Dimensions: []*cloudwatch.Dimension{
							{
								Name:  aws.String("InstanceId"),
								Value: aws.String(instanceID),
							},
						},
						MetricName: aws.String("CPUUtilization"),
						Namespace:  aws.String(elmType),
					},
					Period: aws.Int64(300),
					Stat:   aws.String(statistic),
				},
			},
		},
	}
	if cloudWatchClient == nil {
		cloudWatchClient = awsclient.GetClient(*clientAuth, awsclient.CLOUDWATCH).(*cloudwatch.CloudWatch)
	}

	result, err := cloudWatchClient.GetMetricData(input)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func init() {
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("elementId", "", "element id")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("elementType", "", "element type")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("query", "", "query")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("cmdbApiUrl", "", "cmdb api")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("vaultUrl", "", "vault end point")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("vaultToken", "", "vault token")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("zone", "", "aws region")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("accessKey", "", "aws access key")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("secretKey", "", "aws secret key")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("crossAccountRoleArn", "", "aws cross account role arn")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("externalId", "", "aws external id")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("cloudWatchQueries", "", "aws cloudwatch metric queries")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("instanceId", "", "instance id")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("startTime", "", "start time")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("endTime", "", "endcl time")
	AwsxEc2CpuUtilizationgraphCmd.PersistentFlags().String("responseType", "", "response type. json/frame")
}
