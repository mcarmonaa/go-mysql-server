package test

import "gopkg.in/src-d/go-mysql-server.v0/sql"

type query struct {
	name           string
	statement      string
	expectedSchema sql.Schema
	expectedRows   int
	expectedErr    bool
}

var queries []*query = []*query{
	&query{
		name:        "All commits in HEAD's histories",
		statement:   query1,
		expectedErr: false,
	},
	&query{
		name:        "All commits referenced by HEAD",
		statement:   query2,
		expectedErr: false,
	},
	&query{
		name:        "All commits in HEAD's histories (until 4 previous commits)",
		statement:   query3,
		expectedErr: false,
	},
	&query{name: "Number of blobs per commit", statement: query4, expectedErr: false},
	&query{
		name:        "Number of blobs per commit per repository in the history of the commits referenced by master",
		statement:   query5,
		expectedErr: false,
	},
	&query{
		name:        "Number of commits per month per user and pe repo in year 2017",
		statement:   query6,
		expectedErr: false,
	},
	&query{
		name:        "Commits pointed by more than one references",
		statement:   query7,
		expectedErr: false,
	},
	&query{
		name:        "Number of projects created per year",
		statement:   query8,
		expectedErr: false,
	},
	&query{
		name:        "Number of committer per project",
		statement:   query9,
		expectedErr: false,
	},
}

const (
	query1 = `SELECT * FROM commits INNER JOIN refs ON historyidx(refs.name, commits.hash) >= 0 AND refs.name = 'HEAD';`

	query2 = `SELECT * FROM commits INNER JOIN refs ON refs.hash = commits.hash WHERE refs.name = 'HEAD';`

	query3 = `
    SELECT
	refs.repositoryid,
	refs.name,
	refs.hash AS refcommithash,
	commits.hash AS commithash
    FROM
	commits
    INNER JOIN
	refs
    ON
	historyidx(refs.name, commits.hash) BETWEEN 0 AND 4
    WHERE
	refs.name = 'HEAD';`

	query4 = `
    SELECT
	commits.hash AS commithash,
	COUNT(blobs.hash) AS blobsamount
    FROM
	commits
    INNER JOIN
	blobs
    ON
	commitcontains(commits.hash, blobs.hash)
    GROUP BY
	commits.hash;`

	query5 = `
    SELECT
	refs.repositoryid AS repositoryid,
	commits.hash AS commithash,
	COUNT(blobs.hash) AS blobamount
    FROM
	refs
    INNER JOIN
	commits ON historyidx(refs.hash, commits.hash) AND refs.name = 'refs/head/master'
    INNER JOIN
	blobs ON commitcontains(commits.hash, blobs.hash)
    GROUP BY
	refs.repositoryid,commits.hash;`

	query6 = `
    SELECT
	refs.repositoryid AS repositoryid,
	commits.committeremail AS committer,
	commits.hash AS commithash,
	COUNT(CASE WHEN month(commits.committerdate) = 1 THEN 1 ELSE NULL END) AS january,
	COUNT(CASE WHEN month(commits.committerdate) = 2 THEN 1 ELSE NULL END) AS february,
	COUNT(CASE WHEN month(commits.committerdate) = 3 THEN 1 ELSE NULL END) AS march,
	COUNT(CASE WHEN month(commits.committerdate) = 4 THEN 1 ELSE NULL END) AS april,
	COUNT(CASE WHEN month(commits.committerdate) = 5 THEN 1 ELSE NULL END) AS may,
	COUNT(CASE WHEN month(commits.committerdate) = 6 THEN 1 ELSE NULL END) AS june,
	COUNT(CASE WHEN month(commits.committerdate) = 7 THEN 1 ELSE NULL END) AS july,
	COUNT(CASE WHEN month(commits.committerdate) = 8 THEN 1 ELSE NULL END) AS august,
	COUNT(CASE WHEN month(commits.committerdate) = 9 THEN 1 ELSE NULL END) AS september,
	COUNT(CASE WHEN month(commits.committerdate) = 10 THEN 1 ELSE NULL END) AS october,
	COUNT(CASE WHEN month(commits.committerdate) = 11 THEN 1 ELSE NULL END) AS november,
	COUNT(CASE WHEN month(commits.committerdate) = 11 THEN 1 ELSE NULL END) AS december
    FROM
	commits
    INNER JOIN
	refs ON historyidx(refs.name, commits.hash) >= 0 AND year(commits.committerdate) = 2017
    GROUP BY
	refs.repositoryid, commits.committeremail, commits.hash;`

	query7 = `
    SELECT
	refs.repositoryid AS repositoryid,
	refs.hash AS commithash,
	COUNT(refs.name) AS refsamount
    FROM
	refs
    GROUP BY
	refs.repositoryid, refs.hash
    HAVING
	COUNT(refs.name) > 1;`

	query8 = `
    SELECT
	min(year(commits.committerdate)) AS year,
	COUNT(DISTINCT(refs.repositoryid)) AS reposamount
    FROM
	refs
    INNER JOIN
	commits ON historyidx(refs.hash, commits.hash) AND refs.name = 'refs/head/master'
    GROUP BY
	min(year(commits.committerdate));`

	query9 = `
    SELECT
	refs.repositoryid AS repositoryid,
	COUNT(DISTINCT(commits.authorname)) AS committersamount
    FROM
	refs
    INNER JOIN
	commits ON historyidx(refs.hash, commits.hash) AND refs.name = 'refs/head/master'
    GROUP BY
	refs.repositoryid
    ORDER BY
	COUNT(DISTINCT(commits.authorname)) DESC;`
)
