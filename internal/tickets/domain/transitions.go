package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

func NewTicket(
	accountID int,
	authorUsername, category, subject, body string,
	now time.Time,
) (Ticket, Message, error) {
	cleanSubject, err := ValidateSubject(subject)
	if err != nil {
		return Ticket{}, Message{}, &ValidationError{Fields: FieldErrors{fieldSubject: err.Error()}}
	}
	cleanBody, err := ValidateBody(body)
	if err != nil {
		return Ticket{}, Message{}, &ValidationError{Fields: FieldErrors{fieldBody: err.Error()}}
	}

	ticket := Ticket{
		AccountID:      accountID,
		AuthorUsername: authorUsername,
		Category:       category,
		Subject:        cleanSubject,
		Status:         StatusOpen,
		LastActor:      ActorPlayer,
		MessageCount:   1,
		LastActivity:   now,
		CreatedAt:      now,
	}
	message := Message{
		AuthorID:   accountID,
		AuthorRole: ActorPlayer,
		Visibility: VisibilityPublic,
		Body:       cleanBody,
		CreatedAt:  now,
	}

	return ticket, message, nil
}

func (t Ticket) AppendPublic(authorID int, role Actor, body string, now time.Time) (Ticket, Message, error) {
	if t.IsTerminal() {
		return Ticket{}, Message{}, ErrTicketTerminal
	}
	if role == ActorPlayer && !t.CanPlayerReply() {
		return Ticket{}, Message{}, ErrPlayerCannotReply
	}
	cleanBody, err := ValidateBody(body)
	if err != nil {
		return Ticket{}, Message{}, &ValidationError{Fields: FieldErrors{fieldBody: err.Error()}}
	}

	updated := t
	updated.LastActor = role
	updated.MessageCount = t.MessageCount + 1
	updated.LastActivity = now
	message := Message{
		TicketID:   t.ID,
		AuthorID:   authorID,
		AuthorRole: role,
		Visibility: VisibilityPublic,
		Body:       cleanBody,
		CreatedAt:  now,
	}

	return updated, message, nil
}

func (t Ticket) AppendInternalNote(staffID int, body string, now time.Time) (Message, error) {
	if t.IsTerminal() {
		return Message{}, ErrTicketTerminal
	}
	cleanBody, err := ValidateBody(body)
	if err != nil {
		return Message{}, &ValidationError{Fields: FieldErrors{fieldBody: err.Error()}}
	}

	return Message{
		TicketID:   t.ID,
		AuthorID:   staffID,
		AuthorRole: ActorStaff,
		Visibility: VisibilityInternal,
		Body:       cleanBody,
		CreatedAt:  now,
	}, nil
}

func systemMessage(ticketID int64, staffID int, eventPayload any, fallback string, now time.Time) (Message, error) {
	raw, err := json.Marshal(eventPayload)
	if err != nil {
		return Message{}, fmt.Errorf("domain.systemMessage: %w", err)
	}

	return Message{
		TicketID:   ticketID,
		AuthorID:   staffID,
		AuthorRole: ActorStaff,
		Visibility: VisibilitySystem,
		Body:       fallback,
		Event:      raw,
		CreatedAt:  now,
	}, nil
}

func (t Ticket) Recategorize(staffID int, newCategory string, now time.Time) (Ticket, Message, error) {
	if t.IsTerminal() {
		return Ticket{}, Message{}, ErrTicketTerminal
	}
	if newCategory == t.Category {
		return Ticket{}, Message{}, ErrCategoryUnchanged
	}

	fallback := "Category changed from " + t.Category + " to " + newCategory
	payload := struct {
		Type string `json:"type"`
		From string `json:"from"`
		To   string `json:"to"`
	}{"recategorize", t.Category, newCategory}
	message, err := systemMessage(t.ID, staffID, payload, fallback, now)
	if err != nil {
		return Ticket{}, Message{}, err
	}

	updated := t
	updated.Category = newCategory
	updated.LastActivity = now

	return updated, message, nil
}

func (t Ticket) EditSubject(staffID int, newSubject string, now time.Time) (Ticket, Message, error) {
	if t.IsTerminal() {
		return Ticket{}, Message{}, ErrTicketTerminal
	}
	cleanSubject, err := ValidateSubject(newSubject)
	if err != nil {
		return Ticket{}, Message{}, &ValidationError{Fields: FieldErrors{fieldSubject: err.Error()}}
	}
	if cleanSubject == t.Subject {
		return Ticket{}, Message{}, ErrSubjectUnchanged
	}

	fallback := "Subject changed"
	payload := struct {
		Type string `json:"type"`
		From string `json:"from"`
		To   string `json:"to"`
	}{"subject_edit", t.Subject, cleanSubject}
	message, err := systemMessage(t.ID, staffID, payload, fallback, now)
	if err != nil {
		return Ticket{}, Message{}, err
	}

	updated := t
	updated.Subject = cleanSubject
	updated.LastActivity = now

	return updated, message, nil
}

func (t Ticket) Resolve(staffID int, now time.Time) (Ticket, Message, error) {
	return t.terminate(staffID, StatusResolved, "resolve", "Ticket resolved", now)
}

func (t Ticket) Close(staffID int, now time.Time) (Ticket, Message, error) {
	return t.terminate(staffID, StatusClosed, "close", "Ticket closed", now)
}

func (t Ticket) terminate(staffID int, status Status, eventType, fallback string, now time.Time) (Ticket, Message, error) {
	if t.IsTerminal() {
		return Ticket{}, Message{}, ErrTicketTerminal
	}

	payload := struct {
		Type string `json:"type"`
	}{eventType}
	message, err := systemMessage(t.ID, staffID, payload, fallback, now)
	if err != nil {
		return Ticket{}, Message{}, err
	}

	updated := t
	updated.Status = status
	updated.ClosedBy = &staffID
	updated.LastActivity = now

	return updated, message, nil
}
