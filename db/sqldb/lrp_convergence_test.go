package sqldb_test

import (
	"fmt"
	"sort"
	"time"

	"code.cloudfoundry.org/auctioneer"
	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/bbs/models/test/model_helpers"
	"code.cloudfoundry.org/bbs/test_helpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("LRPConvergence", func() {
	var (
		freshDomain      string
		expiredDomain    string
		evacuatingDomain string
		cellSet          models.CellSet

		sender *fake.FakeMetricSender
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)

		var err error
		freshDomain = "fresh-domain"
		expiredDomain = "expired-domain"
		evacuatingDomain = "evacuating-domain"
		cellSet = models.NewCellSetFromList([]*models.CellPresence{
			{CellId: "existing-cell"},
		})

		createConvergeableScenarios := func(domain string, evacuating bool) {
			var processGuid string
			var instanceGuid string
			processGuid = "desired-with-stale-actuals" + "-" + domain
			desiredLRPWithStaleActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithStaleActuals.Domain = domain
			desiredLRPWithStaleActuals.Instances = 2
			err = sqlDB.DesireLRP(logger, desiredLRPWithStaleActuals)
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(-models.StaleUnclaimedActualLRPDuration)
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			fakeClock.Increment(models.StaleUnclaimedActualLRPDuration)
			queryStr := `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())

			processGuid = "desired-with-missing-cell-actuals" + "-" + domain
			desiredLRPWithMissingCellActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithMissingCellActuals.Domain = domain
			err = sqlDB.DesireLRP(logger, desiredLRPWithMissingCellActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: "actual-with-missing-cell" + "-" + domain, CellId: "missing-cell"})
			Expect(err).NotTo(HaveOccurred())
			queryStr = `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())

			processGuid = "desired-with-extra-actuals" + "-" + domain
			desiredLRPWithExtraActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithExtraActuals.Domain = domain
			desiredLRPWithExtraActuals.Instances = 1
			err = sqlDB.DesireLRP(logger, desiredLRPWithExtraActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: "not-extra-actual" + "-" + domain, CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 1, &models.ActualLRPInstanceKey{InstanceGuid: "extra-actual" + "-" + domain, CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())
			queryStr = `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())

			processGuid = "desired-with-missing-all-actuals" + "-" + domain
			desiredLRPWithMissingAllActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithMissingAllActuals.Domain = domain
			desiredLRPWithMissingAllActuals.Instances = 1
			err = sqlDB.DesireLRP(logger, desiredLRPWithMissingAllActuals)
			Expect(err).NotTo(HaveOccurred())
			queryStr = `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())

			processGuid = "desired-with-missing-some-actuals" + "-" + domain
			desiredLRPWithMissingSomeActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithMissingSomeActuals.Domain = domain
			desiredLRPWithMissingSomeActuals.Instances = 4
			err = sqlDB.DesireLRP(logger, desiredLRPWithMissingSomeActuals)
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 2, Domain: domain}, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			queryStr = `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())

			processGuid = "desired-with-restartable-crashed-actuals" + "-" + domain
			desiredLRPWithRestartableCrashedActuals := model_helpers.NewValidDesiredLRP(processGuid)
			desiredLRPWithRestartableCrashedActuals.Domain = domain
			desiredLRPWithRestartableCrashedActuals.Instances = 1
			err = sqlDB.DesireLRP(logger, desiredLRPWithRestartableCrashedActuals)
			Expect(err).NotTo(HaveOccurred())
			crashedActualLRPKey := &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, crashedActualLRPKey, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			instanceGuid = "restartable-crashed-actual" + "-" + domain
			_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"})
			Expect(err).NotTo(HaveOccurred())
			actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", models.NewPortMapping(2222, 4444))
			_, _, err = sqlDB.StartActualLRP(logger, crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, &actualLRPNetInfo)
			Expect(err).NotTo(HaveOccurred())
			_, _, _, err = sqlDB.CrashActualLRP(logger, crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, "because it failed")
			Expect(err).NotTo(HaveOccurred())
			queryStr = `
				UPDATE actual_lrps
				SET state = ?
				WHERE process_guid = ? AND instance_index = ? AND evacuating = ?
			`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, models.ActualLRPStateCrashed, processGuid, 0, false)
			Expect(err).NotTo(HaveOccurred())
			queryStr = `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())

			processGuid = "actual-with-no-desired" + "-" + domain
			actualLRPWithNoDesired := &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}
			_, err = sqlDB.CreateUnclaimedActualLRP(logger, actualLRPWithNoDesired, processGuid+"-initial")
			Expect(err).NotTo(HaveOccurred())
			queryStr = `UPDATE actual_lrps SET evacuating = ? WHERE process_guid = ?`
			if test_helpers.UsePostgres() {
				queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
			}
			_, err = db.Exec(queryStr, evacuating, processGuid)
			Expect(err).NotTo(HaveOccurred())
		}

		sqlDB.UpsertDomain(logger, freshDomain, 100)
		sqlDB.UpsertDomain(logger, evacuatingDomain, 100)
		fakeClock.Increment(-10 * time.Second)
		sqlDB.UpsertDomain(logger, expiredDomain, 5)
		fakeClock.Increment(10 * time.Second)

		createConvergeableScenarios(freshDomain, false)
		createConvergeableScenarios(expiredDomain, false)
		createConvergeableScenarios(evacuatingDomain, true)

		domain := freshDomain

		processGuid := "normal-desired-lrp" + "-" + domain
		normalDesiredLRP := model_helpers.NewValidDesiredLRP(processGuid)
		normalDesiredLRP.Domain = domain
		normalDesiredLRP.Instances = 2
		err = sqlDB.DesireLRP(logger, normalDesiredLRP)
		Expect(err).NotTo(HaveOccurred())
		_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
		Expect(err).NotTo(HaveOccurred())
		_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain}, processGuid+"-initial")
		Expect(err).NotTo(HaveOccurred())
		_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: "normal-actual-1" + "-" + domain, CellId: "existing-cell"})
		Expect(err).NotTo(HaveOccurred())
		_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 1, &models.ActualLRPInstanceKey{InstanceGuid: "normal-actual-2" + "-" + domain, CellId: "existing-cell"})
		Expect(err).NotTo(HaveOccurred())
		actualLRPNetInfo := models.NewActualLRPNetInfo("some-address", models.NewPortMapping(2222, 4444))
		_, _, err = sqlDB.StartActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain}, &models.ActualLRPInstanceKey{InstanceGuid: "normal-actual-2" + "-" + freshDomain, CellId: "existing-cell"}, &actualLRPNetInfo)
		Expect(err).NotTo(HaveOccurred())

		processGuid = "normal-desired-lrp-with-unclaimed-actuals" + "-" + domain
		normalDesiredLRPWithUnclaimedActuals := model_helpers.NewValidDesiredLRP(processGuid)
		normalDesiredLRPWithUnclaimedActuals.Domain = domain
		normalDesiredLRPWithUnclaimedActuals.Instances = 1
		err = sqlDB.DesireLRP(logger, normalDesiredLRPWithUnclaimedActuals)
		Expect(err).NotTo(HaveOccurred())
		_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
		Expect(err).NotTo(HaveOccurred())

		processGuid = "desired-with-non-restartable-crashed-actuals" + "-" + domain
		desiredLRPWithNonRestartableCrashedActuals := model_helpers.NewValidDesiredLRP(processGuid)
		desiredLRPWithNonRestartableCrashedActuals.Domain = domain
		desiredLRPWithNonRestartableCrashedActuals.Instances = 2
		err = sqlDB.DesireLRP(logger, desiredLRPWithNonRestartableCrashedActuals)
		Expect(err).NotTo(HaveOccurred())
		crashedActualLRPKey := &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}
		_, err = sqlDB.CreateUnclaimedActualLRP(logger, crashedActualLRPKey, processGuid+"-initial")
		Expect(err).NotTo(HaveOccurred())
		instanceGuid := "non-restartable-crashed-actual" + "-" + domain
		_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 0, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"})
		Expect(err).NotTo(HaveOccurred())
		actualLRPNetInfo = models.NewActualLRPNetInfo("some-address", models.NewPortMapping(2222, 4444))
		_, _, err = sqlDB.StartActualLRP(logger, crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, &actualLRPNetInfo)
		Expect(err).NotTo(HaveOccurred())
		_, _, _, err = sqlDB.CrashActualLRP(logger, crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, "because it failed")
		Expect(err).NotTo(HaveOccurred())
		queryStr := `
			UPDATE actual_lrps
			SET crash_count = ?, state = ?
			WHERE process_guid = ? AND instance_index = ? AND evacuating = ?
			`
		if test_helpers.UsePostgres() {
			queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
		}
		_, err = db.Exec(queryStr, models.DefaultMaxRestarts+1, models.ActualLRPStateCrashed, processGuid, 0, false)
		Expect(err).NotTo(HaveOccurred())
		crashedActualLRPKey = &models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: domain}
		_, err = sqlDB.CreateUnclaimedActualLRP(logger, crashedActualLRPKey, processGuid+"-initial")
		Expect(err).NotTo(HaveOccurred())
		instanceGuid = "non-restartable-crashed-actual-2" + "-" + domain
		_, _, err = sqlDB.ClaimActualLRP(logger, processGuid, 1, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"})
		Expect(err).NotTo(HaveOccurred())
		actualLRPNetInfo = models.NewActualLRPNetInfo("some-address", models.NewPortMapping(2222, 4444))
		_, _, err = sqlDB.StartActualLRP(logger, crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, &actualLRPNetInfo)
		Expect(err).NotTo(HaveOccurred())
		_, _, _, err = sqlDB.CrashActualLRP(logger, crashedActualLRPKey, &models.ActualLRPInstanceKey{InstanceGuid: instanceGuid, CellId: "existing-cell"}, "because it failed")
		Expect(err).NotTo(HaveOccurred())
		queryStr = `
			UPDATE actual_lrps
			SET crash_count = ?, state = ?
			WHERE process_guid = ? AND instance_index = ? AND evacuating = ?
			`
		if test_helpers.UsePostgres() {
			queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
		}
		_, err = db.Exec(queryStr, models.DefaultMaxRestarts+1, models.ActualLRPStateCrashed, processGuid, 1, false)
		Expect(err).NotTo(HaveOccurred())

		processGuid = "expired-evacuating-actual-lrp"
		_, err = sqlDB.CreateUnclaimedActualLRP(logger, &models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: domain}, processGuid+"-initial")
		Expect(err).NotTo(HaveOccurred())
		queryStr = `UPDATE actual_lrps SET evacuating = ?, expire_time = ? WHERE process_guid = ?`
		if test_helpers.UsePostgres() {
			queryStr = test_helpers.ReplaceQuestionMarks(queryStr)
		}
		_, err = db.Exec(queryStr, true, fakeClock.Now().UnixNano(), processGuid)
		Expect(err).NotTo(HaveOccurred())

		fakeClock.Increment(1 * time.Second)
	})

	Describe("general metrics", func() {
		It("emits a metric for domains", func() {
			sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(sender.GetValue("Domain." + freshDomain).Value).To(Equal(float64(1)))
		})

		It("emits metrics for lrps", func() {
			convergenceLogger := lagertest.NewTestLogger("convergence")
			sqlDB.ConvergeLRPs(convergenceLogger, cellSet)
			Expect(sender.GetValue("LRPsDesired").Value).To(Equal(float64(35)))
			Expect(sender.GetValue("LRPsClaimed").Value).To(Equal(float64(7)))
			Expect(sender.GetValue("LRPsUnclaimed").Value).To(Equal(float64(29))) // 15 fresh + 4 expired + 10 evac
			Expect(sender.GetValue("LRPsRunning").Value).To(Equal(float64(1)))
			Expect(sender.GetValue("CrashedActualLRPs").Value).To(Equal(float64(2)))
			Expect(sender.GetValue("CrashingDesiredLRPs").Value).To(Equal(float64(1)))
			Consistently(convergenceLogger).ShouldNot(gbytes.Say("failed-.*"))
		})

		It("emits missing LRP metrics", func() {
			sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(sender.GetValue("LRPsMissing").Value).To(Equal(float64(16)))
		})

		It("emits extra LRP metrics", func() {
			sqlDB.ConvergeLRPs(logger, cellSet)
			Expect(sender.GetValue("LRPsExtra").Value).To(Equal(float64(2)))
		})
	})

	Describe("convergence counters", func() {
		It("bumps the convergence counter", func() {
			Expect(sender.GetCounter("ConvergenceLRPRuns")).To(Equal(uint64(0)))
			sqlDB.ConvergeLRPs(logger, models.CellSet{})
			Expect(sender.GetCounter("ConvergenceLRPRuns")).To(Equal(uint64(1)))
			sqlDB.ConvergeLRPs(logger, models.CellSet{})
			Expect(sender.GetCounter("ConvergenceLRPRuns")).To(Equal(uint64(2)))
		})

		It("reports the duration that it took to converge", func() {
			sqlDB.ConvergeLRPs(logger, models.CellSet{})

			reportedDuration := sender.GetValue("ConvergenceLRPDuration")
			Expect(reportedDuration.Unit).To(Equal("nanos"))
			Expect(reportedDuration.Value).NotTo(BeZero())
		})
	})

	It("returns start requests for stale unclaimed actual LRPs", func() {
		startRequests, _, _ := sqlDB.ConvergeLRPs(logger, cellSet)

		By("fresh domain", func() {
			Expect(startRequests).NotTo(BeEmpty())

			processGuid := "desired-with-stale-actuals" + "-" + freshDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 0, 1)

			for _, startRequest := range startRequests {
				sort.Ints(startRequest.Indices)
			}

			Expect(startRequests).To(ContainElement(&lrpStartRequest))
		})

		By("expired domain", func() {
			Expect(startRequests).NotTo(BeEmpty())

			processGuid := "desired-with-stale-actuals" + "-" + expiredDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 0, 1)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))
		})
	})

	It("returns the start requests and actual lrp keys for actuals with missing cells", func() {
		_, keysWithMissingCells, _ := sqlDB.ConvergeLRPs(logger, cellSet)

		By("fresh domain", func() {
			processGuid := "desired-with-missing-cell-actuals" + "-" + freshDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
			Expect(err).NotTo(HaveOccurred())
			expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
			Expect(keysWithMissingCells).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
				Key:            &actualLRPGroup.Instance.ActualLRPKey,
				SchedulingInfo: &expectedSched,
			}))
		})

		By("expired domain", func() {
			processGuid := "desired-with-missing-cell-actuals" + "-" + expiredDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
			Expect(err).NotTo(HaveOccurred())
			expectedSched := desiredLRP.DesiredLRPSchedulingInfo()
			Expect(keysWithMissingCells).To(ContainElement(&models.ActualLRPKeyWithSchedulingInfo{
				Key:            &actualLRPGroup.Instance.ActualLRPKey,
				SchedulingInfo: &expectedSched,
			}))
		})
	})

	It("creates actual LRPs with missing indices, and returns it to be started", func() {
		startRequests, _, _ := sqlDB.ConvergeLRPs(logger, cellSet)
		Expect(startRequests).NotTo(BeEmpty())

		By("missing all actuals, fresh domain", func() {
			processGuid := "desired-with-missing-all-actuals" + "-" + freshDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 0)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})

		By("missing some actuals, fresh domain", func() {
			processGuid := "desired-with-missing-some-actuals" + "-" + freshDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 1, 3)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))

			actualLRPGroup, err = sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})

		By("missing all actuals, expired domain", func() {
			processGuid := "desired-with-missing-all-actuals" + "-" + expiredDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 0)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})

		By("missing some actuals, expired domain", func() {
			processGuid := "desired-with-missing-some-actuals" + "-" + expiredDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 1, 3)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))

			actualLRPGroup, err = sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})
	})

	It("unclaims actual LRPs that are crashed and restartable, and returns it to be started", func() {
		startRequests, _, _ := sqlDB.ConvergeLRPs(logger, cellSet)
		Expect(startRequests).NotTo(BeEmpty())

		By("fresh domain", func() {
			processGuid := "desired-with-restartable-crashed-actuals" + "-" + freshDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 0)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})

		By("expired domain", func() {
			processGuid := "desired-with-restartable-crashed-actuals" + "-" + expiredDomain
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, 0)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
		})
	})

	It("returns extra actual LRPs to be retired", func() {
		_, _, keysToRetire := sqlDB.ConvergeLRPs(logger, cellSet)
		Expect(keysToRetire).NotTo(BeEmpty())

		processGuid := "desired-with-extra-actuals" + "-" + freshDomain
		actualLRPKey := models.ActualLRPKey{ProcessGuid: processGuid, Index: 1, Domain: freshDomain}
		Expect(keysToRetire).To(ContainElement(&actualLRPKey))

		processGuid = "actual-with-no-desired" + "-" + freshDomain
		actualLRPKey = models.ActualLRPKey{ProcessGuid: processGuid, Index: 0, Domain: freshDomain}
		Expect(keysToRetire).To(ContainElement(&actualLRPKey))
	})

	It("creates unclaimed for evacuating instances that are missing the running record", func() {
		startRequests, _, _ := sqlDB.ConvergeLRPs(logger, cellSet)
		Expect(startRequests).NotTo(BeEmpty())

		processGuids := []string{
			"desired-with-stale-actuals" + "-" + evacuatingDomain,
			"desired-with-missing-cell-actuals" + "-" + evacuatingDomain,
			"desired-with-extra-actuals" + "-" + evacuatingDomain,
			"desired-with-missing-all-actuals" + "-" + evacuatingDomain,
			"desired-with-missing-some-actuals" + "-" + evacuatingDomain,
			"desired-with-restartable-crashed-actuals" + "-" + evacuatingDomain,
		}

		for _, processGuid := range processGuids {
			desiredLRP, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())

			indices := []int{}
			for i := 0; i < int(desiredLRP.Instances); i++ {
				indices = append(indices, i)
			}

			lrpStartRequest := auctioneer.NewLRPStartRequestFromModel(desiredLRP, indices...)

			Expect(startRequests).To(ContainElement(&lrpStartRequest))

			for i := 0; i < int(desiredLRP.Instances); i++ {
				actualLRPGroup, err := sqlDB.ActualLRPGroupByProcessGuidAndIndex(logger, processGuid, int32(i))
				Expect(err).NotTo(HaveOccurred())
				Expect(actualLRPGroup.Instance.State).To(Equal(models.ActualLRPStateUnclaimed))
			}
		}
	})

	It("clears out expired domains", func() {
		fetchDomains := func() []string {
			rows, err := db.Query("SELECT domain FROM domains")
			Expect(err).NotTo(HaveOccurred())
			defer rows.Close()

			var domain string
			var results []string
			for rows.Next() {
				err = rows.Scan(&domain)
				Expect(err).NotTo(HaveOccurred())
				results = append(results, domain)
			}
			return results
		}

		Expect(fetchDomains()).To(ContainElement(expiredDomain))

		sqlDB.ConvergeLRPs(logger, cellSet)

		Expect(fetchDomains()).NotTo(ContainElement(expiredDomain))
	})

	It("clears out expired evacuating actual lrps", func() {
		fetchActuals := func() []string {
			rows, err := db.Query("SELECT process_guid FROM actual_lrps")
			Expect(err).NotTo(HaveOccurred())
			defer rows.Close()

			var processGuid string
			var results []string
			for rows.Next() {
				err = rows.Scan(&processGuid)
				Expect(err).NotTo(HaveOccurred())
				results = append(results, processGuid)
			}
			return results
		}

		Expect(fetchActuals()).To(ContainElement("expired-evacuating-actual-lrp"))

		sqlDB.ConvergeLRPs(logger, cellSet)

		Expect(fetchActuals()).NotTo(ContainElement("expired-evacuating-actual-lrp"))
	})

	It("ignores LRPs that don't need convergence", func() {
		processGuids := []string{
			"normal-desired-lrp" + "-" + freshDomain,
			"normal-desired-lrp-with-unclaimed-actuals" + "-" + freshDomain,
			"desired-with-non-restartable-crashed-actuals" + "-" + freshDomain,
			"desired-with-extra-actuals" + "-" + expiredDomain,
		}

		fetch := func(processGuid string) (*models.DesiredLRP, []*models.ActualLRPGroup) {
			desired, err := sqlDB.DesiredLRPByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("should've found desired lrp with guid: %s", processGuid))
			actuals, err := sqlDB.ActualLRPGroupsByProcessGuid(logger, processGuid)
			Expect(err).NotTo(HaveOccurred())
			return desired, actuals
		}

		beforeDesireds := make([]*models.DesiredLRP, 0, len(processGuids))
		beforeActuals := make([][]*models.ActualLRPGroup, 0, len(processGuids))
		for _, processGuid := range processGuids {
			desired, actuals := fetch(processGuid)
			beforeDesireds = append(beforeDesireds, desired)
			beforeActuals = append(beforeActuals, actuals)
		}

		startRequests, keysWithMissingCells, keysToRetire := sqlDB.ConvergeLRPs(logger, cellSet)

		startGuids := make([]string, 0, len(startRequests))
		for _, startRequest := range startRequests {
			startGuids = append(startGuids, startRequest.ProcessGuid)
		}

		for _, processGuid := range processGuids {
			Expect(startGuids).NotTo(ContainElement(processGuid))
		}

		retiredGuids := make([]string, 0, len(keysToRetire))
		for _, keyToRetire := range keysToRetire {
			retiredGuids = append(retiredGuids, keyToRetire.ProcessGuid)
		}
		for _, processGuid := range processGuids {
			Expect(retiredGuids).NotTo(ContainElement(processGuid))
		}

		guidsToUnclaim := make([]string, 0, len(keysWithMissingCells))
		for _, keyWithMissingCell := range keysWithMissingCells {
			guidsToUnclaim = append(guidsToUnclaim, keyWithMissingCell.Key.ProcessGuid)
		}
		for _, processGuid := range processGuids {
			Expect(guidsToUnclaim).NotTo(ContainElement(processGuid))
		}

		afterDesireds := make([]*models.DesiredLRP, 0, len(processGuids))
		afterActuals := make([][]*models.ActualLRPGroup, 0, len(processGuids))
		for _, processGuid := range processGuids {
			desired, actuals := fetch(processGuid)
			afterDesireds = append(afterDesireds, desired)
			afterActuals = append(afterActuals, actuals)
		}

		Expect(beforeDesireds).To(Equal(afterDesireds))
		Expect(beforeActuals).To(Equal(afterActuals))
	})

	Context("when the cell set is empty", func() {
		BeforeEach(func() {
			cellSet = models.NewCellSetFromList([]*models.CellPresence{})
		})

		It("reports all actual lrps as missing cells", func() {
			_, actualsWithMissingCells, _ := sqlDB.ConvergeLRPs(logger, models.CellSet{})
			Expect(len(actualsWithMissingCells)).To(Equal(21))
		})
	})
})
