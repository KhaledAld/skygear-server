package exec

import (
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/oursky/ourd/oddb"
	. "github.com/oursky/ourd/ourtest"
	odplugin "github.com/oursky/ourd/plugin"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRun(t *testing.T) {
	Convey("test args and stdout", t, func() {
		transport := execTransport{
			Path: "/bin/echo",
			Args: []string{},
		}

		originalCommand := startCommand
		defer func() {
			startCommand = originalCommand
		}()

		Convey("init", func() {
			out, err := transport.RunInit()
			So(err, ShouldBeNil)
			So(string(out), ShouldEqual, "init")
		})

		startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
			out, err = originalCommand(cmd, in)
			out = append([]byte(`{"result":"`), out...)
			out = append(out, []byte(`"}`)...)
			return
		}

		Convey("op", func() {
			out, err := transport.RunLambda("hello:world", []byte{})
			So(err, ShouldBeNil)
			So(string(out), ShouldEqual, `"op hello:world"`)
		})

		Convey("handler", func() {
			out, err := transport.RunHandler("hello:world", []byte{})
			So(err, ShouldBeNil)
			So(string(out), ShouldEqual, `"handler hello:world"`)
		})
	})

	Convey("test stdin", t, func() {
		transport := execTransport{
			Path: "/bin/sh",
			Args: []string{"-c", `"cat"`},
		}

		Convey("init", func() {
			out, err := transport.RunInit()
			So(err, ShouldBeNil)
			So(string(out), ShouldEqual, "")
		})

		Convey("op", func() {
			out, err := transport.RunLambda("hello:world", []byte(`{"result": "hello world"}`))
			So(err, ShouldBeNil)
			So(string(out), ShouldEqual, `"hello world"`)
		})

		Convey("handler", func() {
			out, err := transport.RunHandler("hello:world", []byte(`{"result": "hello world"}`))
			So(err, ShouldBeNil)
			So(string(out), ShouldEqual, `"hello world"`)
		})
	})

	Convey("test hook", t, func() {
		transport := execTransport{
			Path: "/never/invoked",
			Args: nil,
		}

		// expect child test case to override startCommand
		// save the original and defer setting it back
		originalCommand := startCommand
		defer func() {
			startCommand = originalCommand
		}()

		recordin := oddb.Record{
			ID:      oddb.NewRecordID("note", "id"),
			OwnerID: "john.doe@example.com",
			ACL: oddb.RecordACL{
				oddb.NewRecordACLEntryRelation("friend", oddb.WriteLevel),
				oddb.NewRecordACLEntryDirect("user_id", oddb.ReadLevel),
			},
			Data: map[string]interface{}{
				"content":   "some note content",
				"noteOrder": float64(1),
				"tags":      []interface{}{"test", "unimportant"},
				"date":      time.Date(2017, 7, 23, 19, 30, 24, 0, time.UTC),
				"ref":       oddb.NewReference("category", "1"),
				"asset":     oddb.Asset{Name: "asset-name"},
			},
		}

		Convey("executes beforeSave correctly", func() {
			called := false
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				called = true
				So(cmd.Path, ShouldEqual, "/never/invoked")
				So(cmd.Args, ShouldResemble, []string{"/never/invoked", "hook", "note:beforeSave"})
				So(in, ShouldEqualJSON, `{
					"_id": "note/id",
					"_ownerID": "john.doe@example.com",
					"content": "some note content",
					"noteOrder": 1,
					"tags": ["test", "unimportant"],
					"date": {
						"$type": "date",
						"$date": "2017-07-23T19:30:24Z"
					},
					"ref": {
						"$type": "ref",
						"$id": "category/1"
					},
					"asset":{
						"$type": "asset",
						"$name": "asset-name"
					},
					"_access": [{
						"relation": "friend",
						"level": "write"
					}, {
						"relation": "$direct",
						"level": "read",
						"user_id": "user_id"
					}]
				}`)

				return []byte(`{
					"result": {
						"_id": "note/id",
						"_ownerID": "john.doe@example.com",
						"content": "content has been modified",
						"noteOrder": 1,
						"tags": ["test", "unimportant"],
						"date": {
							"$type": "date",
							"$date": "2017-07-23T19:30:24Z"
						},
						"ref": {
							"$type": "ref",
							"$id": "category/1"
						},
						"asset":{
							"$type": "asset",
							"$name": "asset-name"
						},
						"_access": [{
							"relation": "friend",
							"level": "write"
						}, {
							"relation": "$direct",
							"level": "read",
							"user_id": "user_id"
						}]
					}
				}`), nil
			}

			recordout, err := transport.RunHook("note", "beforeSave", &recordin)
			So(err, ShouldBeNil)
			So(called, ShouldBeTrue)

			datein := recordin.Data["date"].(time.Time)
			delete(recordin.Data, "date")
			So(recordin, ShouldResemble, oddb.Record{
				ID:      oddb.NewRecordID("note", "id"),
				OwnerID: "john.doe@example.com",
				ACL: oddb.RecordACL{
					oddb.NewRecordACLEntryRelation("friend", oddb.WriteLevel),
					oddb.NewRecordACLEntryDirect("user_id", oddb.ReadLevel),
				},
				Data: map[string]interface{}{
					"content":   "some note content",
					"noteOrder": float64(1),
					"tags":      []interface{}{"test", "unimportant"},
					"ref":       oddb.NewReference("category", "1"),
					"asset":     oddb.Asset{Name: "asset-name"},
				},
			})
			// GoConvey's bug, ShouldEqual and ShouldResemble doesn't work on time.Time
			So(datein == time.Date(2017, 7, 23, 19, 30, 24, 0, time.UTC), ShouldBeTrue)

			dateout := recordout.Data["date"].(time.Time)
			delete(recordout.Data, "date")
			So(*recordout, ShouldResemble, oddb.Record{
				ID:      oddb.NewRecordID("note", "id"),
				OwnerID: "john.doe@example.com",
				ACL: oddb.RecordACL{
					oddb.NewRecordACLEntryRelation("friend", oddb.WriteLevel),
					oddb.NewRecordACLEntryDirect("user_id", oddb.ReadLevel),
				},
				Data: map[string]interface{}{
					"content":   "content has been modified",
					"noteOrder": float64(1),
					"tags":      []interface{}{"test", "unimportant"},
					"ref":       oddb.NewReference("category", "1"),
					"asset":     oddb.Asset{Name: "asset-name"},
				},
			})
			So(dateout == time.Date(2017, 7, 23, 19, 30, 24, 0, time.UTC), ShouldBeTrue)
		})

		Convey("parses null ACL correctly", func() {
			recordin := oddb.Record{
				ID:      oddb.NewRecordID("note", "id"),
				OwnerID: "john.doe@example.com",
				ACL:     nil,
				Data:    map[string]interface{}{},
			}

			called := false
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				called = true
				So(string(in), ShouldEqualJSON, `{
					"_id": "note/id",
					"_ownerID": "john.doe@example.com",
					"_access": null
				}`)
				return []byte(`{
					"result": {
						"_id": "note/id",
						"_ownerID": "john.doe@example.com",
						"_access": null
					}
				}`), nil
			}

			recordout, err := transport.RunHook("note", "beforeSave", &recordin)
			So(err, ShouldBeNil)
			So(called, ShouldBeTrue)
			So(*recordout, ShouldResemble, recordin)
		})

		Convey("returns err if command failed", func() {
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				return nil, errors.New("worrying error")
			}

			recordout, err := transport.RunHook("note", "afterSave", &recordin)
			So(err.Error(), ShouldEqual, "run note:afterSave: worrying error")
			So(recordout, ShouldBeNil)
		})

		Convey("returns err if command returns invalid response", func() {
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				return []byte("I am not a json"), nil
			}

			recordout, err := transport.RunHook("note", "afterSave", &recordin)
			So(err.Error(), ShouldEqual, "run note:afterSave: failed to parse response: invalid character 'I' looking for beginning of value")
			So(recordout, ShouldBeNil)
		})

		Convey("returns err if commands returns with inner error", func() {
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				return []byte(`{
					"result": {
						"ignore": "me"
					},
					"error": {
						"name": "StrongError",
						"desc": "Too strong to lift a feather"
					}
				}`), nil
			}

			recordout, err := transport.RunHook("note", "afterSave", &recordin)
			So(err.Error(), ShouldEqual, `run note:afterSave: StrongError
Too strong to lift a feather`)
			So(recordout, ShouldBeNil)
		})
	})

	Convey("test exec error", t, func() {
		Convey("file not found", func() {
			transport := execTransport{
				Path: "/tmp/nonexistent",
				Args: []string{},
			}

			_, err := transport.RunInit()
			So(err, ShouldNotBeNil)
		})

		Convey("not executable", func() {
			transport := execTransport{
				Path: "/dev/null",
				Args: []string{},
			}

			_, err := transport.RunInit()
			So(err, ShouldNotBeNil)
		})

		Convey("return false", func() {
			transport := execTransport{
				Path: "/bin/false",
				Args: []string{},
			}

			_, err := transport.RunInit()
			So(err, ShouldNotBeNil)
		})
	})

	Convey("test provider", t, func() {
		transport := execTransport{
			Path: "/never/invoked",
			Args: nil,
		}

		// expect child test case to override startCommand
		// save the original and defer setting it back
		originalCommand := startCommand
		defer func() {
			startCommand = originalCommand
		}()

		Convey("executes provider passing auth data", func() {
			called := false
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				called = true
				So(cmd.Path, ShouldEqual, "/never/invoked")
				So(cmd.Args, ShouldResemble, []string{"/never/invoked", "provider", "com.example", "login"})
				So(in, ShouldEqualJSON, `{
					"auth_data": {"password": "secret"}
				}`)

				return []byte(`{
					"result": {
						"principal_id": "johndoe",
						"auth_data": {"token": "A_TOKEN"}
					}
				}`), nil
			}

			authData := map[string]interface{}{
				"password": "secret",
			}
			req := odplugin.AuthRequest{"com.example", "login", authData}

			resp, err := transport.RunProvider(&req)
			So(err, ShouldBeNil)
			So(called, ShouldBeTrue)
			So(resp.PrincipalID, ShouldEqual, "johndoe")
			So(resp.AuthData, ShouldResemble, map[string]interface{}{
				"token": "A_TOKEN",
			})

		})

		Convey("executes provider passing error", func() {
			startCommand = func(cmd *exec.Cmd, in []byte) (out []byte, err error) {
				return nil, errors.New("worrying error")
			}

			authData := map[string]interface{}{}
			req := odplugin.AuthRequest{"com.example", "login", authData}

			resp, err := transport.RunProvider(&req)
			So(err.Error(), ShouldEqual, "run com.example:login: worrying error")
			So(resp, ShouldBeNil)
		})
	})
}

func TestFactory(t *testing.T) {
	Convey("test factory", t, func() {
		factory := execTransportFactory{}
		transport := factory.Open("/bin/echo", []string{"plugin"})

		So(transport, ShouldHaveSameTypeAs, execTransport{})
		So(transport.(execTransport).Path, ShouldResemble, "/bin/echo")
		So(transport.(execTransport).Args, ShouldResemble, []string{"plugin"})
	})
}