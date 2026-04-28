package pubsub_test

import (
	"chatbit/internal/adapter/pubsub"
	"errors"
	"testing"
	"time"

	uuid "github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type PubSubTopicTestSuite struct {
	suite.Suite

	sut *pubsub.AdapterTopic
}

func TestPubSubTopicSuite(t *testing.T) {
	suite.Run(t, new(PubSubTopicTestSuite))
}

func (s *PubSubTopicTestSuite) SetupTest() {
	s.sut = pubsub.NewAdapterTopic()
}

func (s *PubSubTopicTestSuite) TearDownTest() {}

func (s *PubSubTopicTestSuite) TestPubSubTopic() {
	// var
	var someOutput uuid.UUID
	someInput, _ := uuid.NewUUID()
	someTopic, _ := uuid.NewUUID()

	wait := make(chan bool, 1)

	// act
	s.sut.Subscribe(someTopic.String(), func(obj interface{}) {
		someOutput = obj.(uuid.UUID)

		wait <- true
	})

	s.NoError(s.sut.Publish(someTopic.String(), someInput))

	<-wait

	s.Equal(someInput, someOutput)
}

func (s *PubSubTopicTestSuite) TestPubSubTopicUnsubscribe() {
	// var
	var someOutput uuid.UUID
	someInput, _ := uuid.NewUUID()
	someTopic, _ := uuid.NewUUID()

	wait := make(chan bool, 1)

	// act
	subID, err := s.sut.Subscribe(someTopic.String(), func(obj interface{}) {
		someOutput = obj.(uuid.UUID)

		wait <- true
	})
	s.NoError(err)

	s.NoError(s.sut.Publish(someTopic.String(), someInput))

	select {
	case <-wait:
		s.Equal(someInput, someOutput)
	case <-time.After(100 * time.Millisecond):
		s.Require().NoError(errors.New("no subscribe trigger received"))
	}

	s.NoError(s.sut.Unsubscribe(someTopic.String(), subID))

	select {
	case <-wait:
		s.Require().NoError(errors.New("still subscribe trigger received"))
	case <-time.After(100 * time.Millisecond):
	}
}

func (s *PubSubTopicTestSuite) TestPubSubTopicDifferentTopics() {
	// var
	var someOutput uuid.UUID
	someInput, _ := uuid.NewUUID()
	someTopic1, _ := uuid.NewUUID()
	someTopic2, _ := uuid.NewUUID()

	wait := make(chan bool, 1)

	// act
	_, err := s.sut.Subscribe(someTopic1.String(), func(obj interface{}) {
		someOutput = obj.(uuid.UUID)

		wait <- true
	})
	s.NoError(err)

	_, err = s.sut.Subscribe(someTopic2.String(), func(obj interface{}) {
		s.Require().NoError(errors.New("wrong topic"))
	})
	s.NoError(err)

	s.NoError(s.sut.Publish(someTopic1.String(), someInput))

	<-wait

	s.Equal(someInput, someOutput)
}

func (s *PubSubTopicTestSuite) TestPubSubTopicDifferentTopicsSameSub() {
	// var
	var someOutput uuid.UUID
	someInput1, _ := uuid.NewUUID()
	someInput2, _ := uuid.NewUUID()
	someTopic1, _ := uuid.NewUUID()
	someTopic2, _ := uuid.NewUUID()

	wait := make(chan bool, 1)

	// act
	subID, err := s.sut.Subscribe(someTopic1.String(), func(obj interface{}) {
		someOutput = obj.(uuid.UUID)

		wait <- true
	})
	s.NoError(err)

	err = s.sut.AddSubscriberToTopic(someTopic2.String(), subID)
	s.NoError(err)

	s.NoError(s.sut.Publish(someTopic1.String(), someInput1))

	<-wait
	s.Equal(someInput1, someOutput)

	s.NoError(s.sut.Publish(someTopic2.String(), someInput2))
	<-wait
	s.Equal(someInput2, someOutput)

}

func (s *PubSubTopicTestSuite) TestPubSubTopicAfterClose() {
	// var
	var someOutput uuid.UUID
	someInput, _ := uuid.NewUUID()
	someTopic, _ := uuid.NewUUID()

	wait := make(chan bool, 1)

	// act
	_, err := s.sut.Subscribe(someTopic.String(), func(obj interface{}) {
		someOutput = obj.(uuid.UUID)

		wait <- true
	})
	s.NoError(err)

	s.NoError(s.sut.Publish(someTopic.String(), someInput))

	select {
	case <-wait:
		s.Equal(someInput, someOutput)
	case <-time.After(100 * time.Millisecond):
		s.Require().NoError(errors.New("no subscribe trigger received"))
	}

	s.NoError(s.sut.Close())

	s.ErrorIs(s.sut.Publish(someTopic.String(), someInput), pubsub.ErrAlreadyClosed)

	select {
	case <-wait:
		s.Require().NoError(errors.New("still subscribe trigger received"))
	case <-time.After(100 * time.Millisecond):
	}
}

func (s *PubSubTopicTestSuite) TestPubSubTopicPublishToNotExistingTopic() {
	// var
	someInput, _ := uuid.NewUUID()
	someTopic, _ := uuid.NewUUID()

	// act
	s.NoError(s.sut.Publish(someTopic.String(), someInput))
}

func (s *PubSubTopicTestSuite) TestPubSubTopicMultiSubs() {
	// var
	var someOutput1, someOutput2 uuid.UUID
	someInput, _ := uuid.NewUUID()
	someTopic, _ := uuid.NewUUID()

	wait1 := make(chan bool, 1)
	wait2 := make(chan bool, 1)

	// act
	_, err := s.sut.Subscribe(someTopic.String(), func(obj interface{}) {
		someOutput1 = obj.(uuid.UUID)

		wait1 <- true
	})
	s.NoError(err)

	_, err = s.sut.Subscribe(someTopic.String(), func(obj interface{}) {
		someOutput2 = obj.(uuid.UUID)

		wait2 <- true
	})
	s.NoError(err)

	s.NoError(s.sut.Publish(someTopic.String(), someInput))

	<-wait1
	<-wait2

	s.Equal(someInput, someOutput1)

	s.Equal(someInput, someOutput2)
}
