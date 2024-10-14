# TinyamoDB

## Design and Implementation of a Simplified DynamoDB

This document describes the design and implementation of a simplified version of DynamoDB. The following features have been implemented:

- [x] Saving data to local storage
- [x] Partitioned data
- [x] Using the primary key as the partition key
- [x] Overwriting with Put
- [x] Sort key with String

Features not yet implemented:

- [ ] Query by sort key

## How to use

```golang
import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    "github.com/yyyoichi/tinyamodb/tinyamodb"
)

func Example() {
    // init db
    var c tinyamodb.Config
    c.Table.PartitionKey = "gameId" // primary-key
    c.Table.SortKey = "timestamp"   // sork-key

    db, err := tinyamodb.New("/tmp/tinyamodb", c)
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    defer db.Close()

    ctx := context.Background()
    items := []map[string]types.AttributeValue{
        {
            "gameId":    &types.AttributeValueMemberS{Value: "GAME#1"},
            "action":    &types.AttributeValueMemberS{Value: "ATACK"},
            "player":    &types.AttributeValueMemberN{Value: "1"},
            "timestamp": &types.AttributeValueMemberS{Value: "2024/10/14-10:00-0000"},
        },
        {
            "gameId":    &types.AttributeValueMemberS{Value: "GAME#1"},
            "action":    &types.AttributeValueMemberS{Value: "MOVE"},
            "player":    &types.AttributeValueMemberN{Value: "2"},
            "timestamp": &types.AttributeValueMemberS{Value: "2024/10/14-10:01-0000"},
        },
        {
            "gameId":    &types.AttributeValueMemberS{Value: "GAME#1"},
            "action":    &types.AttributeValueMemberS{Value: "ATACK"},
            "player":    &types.AttributeValueMemberN{Value: "1"},
            "timestamp": &types.AttributeValueMemberS{Value: "2024/10/14-10:02-0000"},
        },
        {
            "gameId":    &types.AttributeValueMemberS{Value: "GAME#2"},
            "action":    &types.AttributeValueMemberS{Value: "MOVE"},
            "player":    &types.AttributeValueMemberN{Value: "5"},
            "timestamp": &types.AttributeValueMemberS{Value: "2024/10/14-10:00-0000"},
        },
    }
    for _, item := range items {
        _, err = db.PutItem(ctx, &tinyamodb.PutItemInput{Item: item})
        if err != nil {
            log.Fatalf("Error: %v", err)
        }
    }

    output, err := db.GetItem(ctx, &tinyamodb.GetItemInput{
        Key: map[string]types.AttributeValue{
            "gameId":    &types.AttributeValueMemberS{Value: "GAME#1"},
            "timestamp": &types.AttributeValueMemberS{Value: "2024/10/14-10:00-0000"},
        },
    })
    if err != nil {
        log.Fatalf("Error: %v", err)
    }
    action := output.Item["action"].(*types.AttributeValueMemberS)
    player := output.Item["player"].(*types.AttributeValueMemberN)
    fmt.Println(action.Value)
    fmt.Println(player.Value)
    // Output:
    // ATACK
    // 1
}

```
