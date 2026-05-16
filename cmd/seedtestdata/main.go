// seedtestdata creates a deterministic local-dev fixture so the W3
// leaderboard surfaces can be exercised end-to-end without hand-clicking
// every model into existence.
//
// What it creates (idempotent — safe to re-run):
//
//   - Account id=1, tenant_mode='k5' — exercises the strictest privacy
//     mechanic (pseudonyms always, no top-N for any student).
//   - 2 admins  — admin@paper.test, michael@aprendio.ai / password "paperpaper"
//   - 1 teacher — teacher@paper.test / password "paperpaper"
//   - 7 students at student1..student7@paper.test / "paperpaper"
//   - 1 course "W3 Test Class" with all of them enrolled.
//   - Varied XP transactions so the ranking is meaningful (top student
//     has ~600 xp, bottom has ~20 xp, plus one opted-out student).
//
// Usage:
//
//   DATABASE_URL=postgres://paper:paper@localhost:5433/paper_lms?sslmode=disable \
//       go run ./cmd/seedtestdata
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

const testPassword = "paperpaper"

func main() { os.Exit(run()) }

func run() int {
	_ = godotenv.Load()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		return 2
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Printf("open database: %v", err)
		return 2
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	ctx := context.Background()

	// 1. Account — tenant_mode=k5 so the pseudonym + relative-window
	//    mechanics fire by default for student viewers.
	account := models.Account{
		ID:           1,
		Name:         "Paper LMS Test Tenant",
		WorkflowState: "active",
		TenantMode:   "k5",
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "tenant_mode"}),
	}).Create(&account).Error; err != nil {
		log.Printf("create account: %v", err)
		return 2
	}
	fmt.Printf("✓ account #%d (%s, %s)\n", account.ID, account.Name, account.TenantMode)

	// 2. Seed system currencies for this tenant. The Wave 1 seed lives
	//    in the gamification package and is safe to re-run.
	if err := gamification.SeedSystemCurrenciesForTenant(ctx, db, account.ID); err != nil {
		log.Printf("seed currencies: %v", err)
		return 2
	}
	fmt.Println("✓ system currencies seeded")

	// 3. Users.
	admin, err := upsertUser(db, "admin@paper.test", "Avery Admin", "admin")
	if err != nil {
		log.Printf("admin: %v", err)
		return 2
	}
	// Owner-developer admin account so I can log in as myself without
	// reaching for the generic seed admin.
	ownerAdmin, err := upsertUser(db, "michael@aprendio.ai", "Michael Kocher", "admin")
	if err != nil {
		log.Printf("owner admin: %v", err)
		return 2
	}
	teacher, err := upsertUser(db, "teacher@paper.test", "Taylor Teacher", "user")
	if err != nil {
		log.Printf("teacher: %v", err)
		return 2
	}
	students := []struct {
		login string
		name  string
	}{
		{"student1@paper.test", "Sofia Alvarez"},
		{"student2@paper.test", "Ben Carter"},
		{"student3@paper.test", "Chen Wei"},
		{"student4@paper.test", "Diego Martinez"},
		{"student5@paper.test", "Emma Patel"},
		{"student6@paper.test", "Farah Khalil"},
		{"student7@paper.test", "Gabriel O'Donnell"},
	}
	studentRows := make([]*models.User, 0, len(students))
	for _, s := range students {
		u, err := upsertUser(db, s.login, s.name, "user")
		if err != nil {
			log.Printf("student %s: %v", s.login, err)
			return 2
		}
		studentRows = append(studentRows, u)
	}
	fmt.Printf("✓ %d users (%s + %s admins, %s teacher, %d students)\n", 3+len(studentRows), admin.LoginID, ownerAdmin.LoginID, teacher.LoginID, len(studentRows))

	// 4. One opted-out student so we can verify FilterPublicLeaderboard
	//    drops them from peer views.
	if studentRows[3].LeaderboardOptOut != true {
		studentRows[3].LeaderboardOptOut = true
		if err := db.Save(studentRows[3]).Error; err != nil {
			log.Printf("opt-out student %d: %v", studentRows[3].ID, err)
			return 2
		}
	}
	fmt.Printf("✓ %s opted out of public leaderboards (privacy filter test)\n", studentRows[3].LoginID)

	// 5. Course.
	course := models.Course{
		AccountID:     account.ID,
		Name:          "W3 Test Class",
		CourseCode:    "W3-TEST",
		WorkflowState: "available",
		DefaultView:   "modules",
	}
	if err := db.Where("course_code = ?", course.CourseCode).Attrs(&course).FirstOrCreate(&course).Error; err != nil {
		log.Printf("course: %v", err)
		return 2
	}
	fmt.Printf("✓ course #%d (%s)\n", course.ID, course.Name)

	// 6. Enrollments. Teacher → TeacherEnrollment, students → StudentEnrollment.
	if err := upsertEnrollment(db, teacher.ID, course.ID, "TeacherEnrollment"); err != nil {
		log.Printf("teacher enrollment: %v", err)
		return 2
	}
	for _, s := range studentRows {
		if err := upsertEnrollment(db, s.ID, course.ID, "StudentEnrollment"); err != nil {
			log.Printf("student enrollment %d: %v", s.ID, err)
			return 2
		}
	}
	fmt.Printf("✓ enrollments seeded (1 teacher, %d students)\n", len(studentRows))

	// 7. XP transactions. Varied amounts so the ranking is meaningful.
	//    Numbers chosen so the relative-window mechanic has a clean
	//    "next to beat" gap for each viewer.
	xpAmounts := []int64{640, 520, 440, 360, 250, 140, 40}
	if err := awardXP(ctx, db, account.ID, studentRows, xpAmounts); err != nil {
		log.Printf("award XP: %v", err)
		return 2
	}
	fmt.Printf("✓ XP awarded: %v\n", xpAmounts)

	// 8. Summary.
	fmt.Println()
	fmt.Println("Done. Test logins (password is \"" + testPassword + "\" for all):")
	fmt.Println("  Admin   →", admin.LoginID)
	fmt.Println("  Admin   →", ownerAdmin.LoginID, "(owner)")
	fmt.Println("  Teacher →", teacher.LoginID)
	for i, s := range studentRows {
		mark := ""
		if s.LeaderboardOptOut {
			mark = "  ← opted out of leaderboard"
		}
		fmt.Printf("  Student → %s (%d xp, %s)%s\n", s.LoginID, xpAmounts[i], s.Name, mark)
	}
	return 0
}

// upsertUser creates or fetches a user by login_id. Always (re)sets the
// password so the seed remains a single source of truth.
func upsertUser(db *gorm.DB, login, name, role string) (*models.User, error) {
	user := models.User{LoginID: login}
	err := db.Where("login_id = ?", login).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = models.User{
			LoginID:      login,
			Email:        login,
			Name:         name,
			SortableName: name,
			ShortName:    name,
			Role:         role,
		}
		if err := user.HashPassword(testPassword); err != nil {
			return nil, err
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}
	if err != nil {
		return nil, err
	}
	// Update existing — refresh name + role + password so re-runs
	// stay consistent with the seed source.
	user.Name = name
	user.SortableName = name
	user.ShortName = name
	user.Email = login
	user.Role = role
	if err := user.HashPassword(testPassword); err != nil {
		return nil, err
	}
	if err := db.Save(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func upsertEnrollment(db *gorm.DB, userID, courseID uint, enrollmentType string) error {
	var existing models.Enrollment
	err := db.Where("user_id = ? AND course_id = ? AND type = ?", userID, courseID, enrollmentType).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role := enrollmentType // canvas convention: role mirrors type for default cases
		return db.Create(&models.Enrollment{
			UserID:        userID,
			CourseID:      courseID,
			Type:          enrollmentType,
			Role:          role,
			WorkflowState: "active",
		}).Error
	}
	if err != nil {
		return err
	}
	if existing.WorkflowState != "active" {
		existing.WorkflowState = "active"
		return db.Save(&existing).Error
	}
	return nil
}

// awardXP credits each student with the matching amount. Idempotent
// w.r.t. lifetime_earned: if a student already has >= the target, we
// skip; otherwise top up to the target with a single transaction.
func awardXP(ctx context.Context, db *gorm.DB, accountID uint, students []*models.User, amounts []int64) error {
	var xp models.GamificationCurrencyType
	if err := db.Where("tenant_id = ? AND scope_type = ? AND scope_id = ? AND code = ?",
		accountID, models.ScopeSite, accountID, "xp").First(&xp).Error; err != nil {
		return fmt.Errorf("load xp currency: %w", err)
	}
	for i, s := range students {
		want := amounts[i]
		var bal models.GamificationWalletBalance
		err := db.Where("user_id = ? AND currency_type_id = ?", s.ID, xp.ID).First(&bal).Error
		current := int64(0)
		if err == nil {
			current = bal.LifetimeEarned
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		delta := want - current
		if delta <= 0 {
			continue
		}
		tx := models.GamificationWalletTransaction{
			UserID:         s.ID,
			CurrencyTypeID: xp.ID,
			Delta:          delta,
			Reason:         "seed:test-fixture",
		}
		if err := db.Create(&tx).Error; err != nil {
			return fmt.Errorf("write tx for user %d: %w", s.ID, err)
		}
		bal.UserID = s.ID
		bal.CurrencyTypeID = xp.ID
		bal.Balance += delta
		bal.LifetimeEarned += delta
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&bal).Error; err != nil {
				return fmt.Errorf("create balance for user %d: %w", s.ID, err)
			}
		} else {
			if err := db.Save(&bal).Error; err != nil {
				return fmt.Errorf("save balance for user %d: %w", s.ID, err)
			}
		}
	}
	_ = ctx
	return nil
}
