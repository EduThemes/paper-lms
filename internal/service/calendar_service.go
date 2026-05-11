package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type CalendarService struct {
	repo repository.CalendarEventRepository
}

func NewCalendarService(repo repository.CalendarEventRepository) *CalendarService {
	return &CalendarService{repo: repo}
}

func (s *CalendarService) Create(ctx context.Context, event *models.CalendarEvent) error {
	if event.Title == "" {
		return errors.New("calendar event title is required")
	}
	if event.StartAt.IsZero() {
		return errors.New("calendar event start_at is required")
	}
	if event.WorkflowState == "" {
		event.WorkflowState = "active"
	}
	return s.repo.Create(ctx, event)
}

func (s *CalendarService) GetByID(ctx context.Context, id uint) (*models.CalendarEvent, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *CalendarService) Update(ctx context.Context, event *models.CalendarEvent) error {
	return s.repo.Update(ctx, event)
}

func (s *CalendarService) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *CalendarService) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CalendarEvent], error) {
	return s.repo.ListByContext(ctx, contextType, contextID, params)
}

func (s *CalendarService) ExportAsICalendar(ctx context.Context, contextType string, contextID uint, startAt, endAt time.Time) ([]byte, error) {
	events, err := s.repo.ListByContextAndDateRange(ctx, contextType, contextID, startAt, endAt)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("BEGIN:VCALENDAR\r\n")
	buf.WriteString("VERSION:2.0\r\n")
	buf.WriteString("PRODID:-//Paper LMS//EN\r\n")
	buf.WriteString("CALSCALE:GREGORIAN\r\n")

	for _, e := range events {
		buf.WriteString("BEGIN:VEVENT\r\n")
		buf.WriteString(fmt.Sprintf("UID:event-%d@paper-lms\r\n", e.ID))
		buf.WriteString(fmt.Sprintf("SUMMARY:%s\r\n", escapeICalText(e.Title)))

		if e.Description != "" {
			buf.WriteString(fmt.Sprintf("DESCRIPTION:%s\r\n", escapeICalText(e.Description)))
		}

		if e.AllDay {
			buf.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", e.StartAt.UTC().Format("20060102")))
			if e.EndAt != nil {
				buf.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", e.EndAt.UTC().Format("20060102")))
			}
		} else {
			buf.WriteString(fmt.Sprintf("DTSTART:%s\r\n", e.StartAt.UTC().Format("20060102T150405Z")))
			if e.EndAt != nil {
				buf.WriteString(fmt.Sprintf("DTEND:%s\r\n", e.EndAt.UTC().Format("20060102T150405Z")))
			}
		}

		location := e.LocationName
		if e.LocationAddress != "" {
			if location != "" {
				location += ", "
			}
			location += e.LocationAddress
		}
		if location != "" {
			buf.WriteString(fmt.Sprintf("LOCATION:%s\r\n", escapeICalText(location)))
		}

		buf.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", e.CreatedAt.UTC().Format("20060102T150405Z")))
		buf.WriteString("END:VEVENT\r\n")
	}

	buf.WriteString("END:VCALENDAR\r\n")
	return buf.Bytes(), nil
}

func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
