// practice-pg-command provides a PostgreSQL wire-protocol endpoint backed by
// a real PostgreSQL instance. This keeps psql catalog commands version-safe.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
)

type server struct {
	addr string
	db   *pgxpool.Pool
}

var questions = []string{
	"問題1: テーブル一覧を表示してください。ヒント: \\dt",
	"正解です。\n\n問題2: customers テーブルの定義を表示してください。ヒント: \\d customers",
	"正解です。\n\n問題3: 詳細付きのテーブル一覧を表示してください。ヒント: \\dt+",
	"正解です。\n\n問題4: データベース一覧を表示してください。ヒント: \\l",
	"正解です。\n\n問題5: psql のヘルプを表示してください。ヒント: \\?（これはクライアント側の機能です）",
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":5432"
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://practice:practice@localhost:5433/practice?sslmode=disable"
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("database is unavailable: %v", err)
	}
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("practice PostgreSQL protocol server listening on %s", addr)
	for {
		c, err := l.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go (&server{addr: addr, db: db}).serve(c)
	}
}

func (s *server) serve(c net.Conn) {
	defer c.Close()
	b := pgproto3.NewBackend(bufio.NewReader(c), c)
	for {
		m, err := b.ReceiveStartupMessage()
		if err != nil {
			return
		}
		switch m.(type) {
		case *pgproto3.SSLRequest, *pgproto3.GSSEncRequest:
			_, _ = c.Write([]byte("N"))
			continue
		case *pgproto3.CancelRequest:
			return
		}
		break
	}
	send(b, &pgproto3.AuthenticationOk{})
	for _, p := range []pgproto3.ParameterStatus{{Name: "server_version", Value: "16.0-practice"}, {Name: "server_encoding", Value: "UTF8"}, {Name: "client_encoding", Value: "UTF8"}, {Name: "DateStyle", Value: "ISO, MDY"}, {Name: "standard_conforming_strings", Value: "on"}} {
		send(b, &p)
	}
	send(b, &pgproto3.BackendKeyData{ProcessID: 1, SecretKey: 1})
	send(b, &pgproto3.ReadyForQuery{TxStatus: 'I'})
	question := 0
	for {
		m, err := b.Receive()
		if err != nil {
			return
		}
		switch x := m.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			s.handleQuery(b, x.String, &question)
		default:
			sendError(b, "0A000", "この練習サーバーは simple query protocol のみ対応しています")
			send(b, &pgproto3.ReadyForQuery{TxStatus: 'I'})
		}
	}
}

func (s *server) handleQuery(b *pgproto3.Backend, query string, question *int) {
	q := strings.TrimSpace(strings.TrimSuffix(query, ";"))
	if q == "" {
		send(b, &pgproto3.EmptyQueryResponse{})
		send(b, &pgproto3.ReadyForQuery{TxStatus: 'I'})
		return
	}
	rows, err := s.db.Query(context.Background(), q)
	if err != nil {
		sendError(b, "42601", err.Error())
		send(b, &pgproto3.ReadyForQuery{TxStatus: 'I'})
		return
	}
	if err := sendRows(b, rows); err != nil {
		sendError(b, "XX000", err.Error())
		send(b, &pgproto3.ReadyForQuery{TxStatus: 'I'})
		return
	}
	send(b, &pgproto3.ReadyForQuery{TxStatus: 'I'})
	if shouldAdvanceQuestion(q, *question) {
		advanceQuestion(b, question)
	}
}

func sendRows(b *pgproto3.Backend, rows pgx.Rows) error {
	defer rows.Close()
	fd := rows.FieldDescriptions()
	fields := make([]pgproto3.FieldDescription, len(fd))
	for i, f := range fd {
		fields[i] = pgproto3.FieldDescription{Name: []byte(f.Name), TableOID: f.TableOID, TableAttributeNumber: f.TableAttributeNumber, DataTypeOID: f.DataTypeOID, DataTypeSize: f.DataTypeSize, TypeModifier: f.TypeModifier, Format: 0}
	}
	if len(fields) > 0 {
		send(b, &pgproto3.RowDescription{Fields: fields})
	}
	count := 0
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return err
		}
		encoded := make([][]byte, len(values))
		for i, v := range values {
			if v != nil {
				encoded[i] = []byte(fmt.Sprint(v))
			}
		}
		send(b, &pgproto3.DataRow{Values: encoded})
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	send(b, &pgproto3.CommandComplete{CommandTag: []byte("SELECT " + strconv.Itoa(count))})
	return nil
}

func shouldAdvanceQuestion(q string, question int) bool {
	l := strings.ToLower(q)
	switch question {
	case 0, 2:
		return strings.Contains(l, "pg_catalog.pg_class")
	case 1:
		return strings.Contains(l, "pg_catalog.pg_attribute")
	case 3:
		return strings.Contains(l, "pg_catalog.pg_database")
	default:
		return false
	}
}
func advanceQuestion(b *pgproto3.Backend, q *int) {
	if *q < len(questions)-1 {
		(*q)++
	}
	send(b, (*pgproto3.NoticeResponse)(&pgproto3.ErrorResponse{Severity: "NOTICE", Code: "00000", Message: questions[*q]}))
}
func sendError(b *pgproto3.Backend, code, msg string) {
	send(b, &pgproto3.ErrorResponse{Severity: "ERROR", Code: code, Message: msg})
}
func send(b *pgproto3.Backend, m pgproto3.BackendMessage) { b.Send(m); _ = b.Flush() }
